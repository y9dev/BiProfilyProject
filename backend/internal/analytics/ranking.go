package analytics

import (
	"context"
	"sort"
	"strings"

	"bigroup-backend/internal/httpapi/dto"
)

// Ranking: per characteristic, each population member's latest measurement with
// day <= to, ranked with RANK() (ties share a rank) by score descending; departments
// by their as-of mean, n >= MinSample only.
func (r *repo) Ranking(ctx context.Context, today Date, q RankingQuery) (dto.RankingResponse, error) {
	out := dto.RankingResponse{MinSample: MinSample, Blocks: []dto.RankingBlock{}}
	limit, offset, err := paging(q.Limit, q.Offset)
	if err != nil {
		return out, err
	}
	from, to, err := r.window(ctx, today, q.Period, true)
	if err != nil {
		return out, err
	}
	depts, err := r.departments(ctx)
	if err != nil {
		return out, err
	}
	if q.Department != nil {
		if err := depts.check("departmentId", *q.Department); err != nil {
			return out, err
		}
	}
	cat, err := r.characteristics(ctx)
	if err != nil {
		return out, err
	}
	chars, err := r.selection(ctx, cat, q.CharacteristicIDs, from, to)
	if err != nil {
		return out, err
	}
	out.AsOf = to.String()
	cids := cidsOf(chars)
	f := popFilter{Dept: q.Department}

	type entry struct {
		dto.RankingEmployee
		dk int64
	}
	byChar := map[int64][]entry{}
	a := args{cids, to.String()}
	rows, err := r.pool.Query(ctx, `
		WITH l AS (
		  SELECT DISTINCT ON (t.user_id, s.characteristic_id)
		         t.user_id, s.characteristic_id AS cid, s.score, t.completed_at
		    FROM users u JOIN attempts t ON t.user_id = u.id JOIN attempt_scores s ON s.attempt_id = t.id
		   WHERE `+popSQL+f.cond(&a)+` AND s.characteristic_id = ANY($1::bigint[])
		     AND t.completed_at < `+endOf("$2")+`
		   ORDER BY t.user_id, s.characteristic_id, t.completed_at DESC, t.id DESC)
		SELECT l.cid, rank() OVER (PARTITION BY l.cid ORDER BY l.score DESC)::int,
		       u.id, u.full_name, COALESCE(u.department_id, 0), l.score::float8, l.completed_at
		  FROM l JOIN users u ON u.id = l.user_id
		 ORDER BY l.cid, l.score DESC, u.full_name, u.id`, a...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var cid int64
		var e entry
		if err := rows.Scan(&cid, &e.Rank, &e.User.ID, &e.User.FullName, &e.dk, &e.Score, &e.MeasuredAt); err != nil {
			rows.Close()
			return out, err
		}
		dk := e.dk
		e.Department = depts.ref(&dk)
		byChar[cid] = append(byChar[cid], e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	// Attempts measuring each characteristic with day <= to, per department.
	attempts := map[groupKey]int{}
	a = args{cids, to.String()}
	rows, err = r.pool.Query(ctx, `
		SELECT COALESCE(u.department_id, 0), s.characteristic_id, count(*)::int
		  FROM users u JOIN attempts t ON t.user_id = u.id JOIN attempt_scores s ON s.attempt_id = t.id
		 WHERE `+popSQL+f.cond(&a)+` AND s.characteristic_id = ANY($1::bigint[])
		   AND t.completed_at < `+endOf("$2")+`
		 GROUP BY 1, 2`, a...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var k groupKey
		var n int
		if err := rows.Scan(&k.dk, &k.cid, &n); err != nil {
			rows.Close()
			return out, err
		}
		attempts[k] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	for _, ch := range chars {
		es := byChar[ch.ID]
		b := dto.RankingBlock{
			Characteristic: ch, Insufficient: len(es) < MinSample,
			Employees:   dto.RankingEmployees{Total: len(es), Items: []dto.RankingEmployee{}},
			Departments: []dto.RankingDepartment{},
		}
		if !b.Insufficient {
			for _, e := range page(es, limit, offset) {
				b.Employees.Items = append(b.Employees.Items, e.RankingEmployee)
			}
		}
		type dsum struct {
			sum float64
			n   int
		}
		sums := map[int64]*dsum{}
		for _, e := range es {
			s := sums[e.dk]
			if s == nil {
				s = &dsum{}
				sums[e.dk] = s
			}
			s.sum += e.Score
			s.n++
		}
		for dk, s := range sums {
			if s.n < MinSample {
				continue
			}
			k := dk
			b.Departments = append(b.Departments, dto.RankingDepartment{
				Department: depts.ref(&k), Value: round2(s.sum / float64(s.n)),
				Employees: s.n, Attempts: attempts[groupKey{dk, ch.ID}],
			})
		}
		sort.Slice(b.Departments, func(i, j int) bool {
			x, y := b.Departments[i], b.Departments[j]
			if x.Value != y.Value {
				return x.Value > y.Value
			}
			return deptName(x.Department) < deptName(y.Department)
		})
		for i := range b.Departments {
			if i > 0 && b.Departments[i].Value == b.Departments[i-1].Value {
				b.Departments[i].Rank = b.Departments[i-1].Rank
			} else {
				b.Departments[i].Rank = i + 1
			}
		}
		out.Blocks = append(out.Blocks, b)
	}
	return out, nil
}

// deptName orders departments by name with "No department" last.
func deptName(d *dto.AnalyticsDepartmentRef) string {
	if d == nil {
		return "￿"
	}
	return strings.ToLower(d.Name)
}

// Changes: consecutive measurement pairs (by completed_at, attempt id) of population
// members where the later one falls in [from, to] and |delta| >= threshold, sorted by
// |delta| descending (then latest first).
func (r *repo) Changes(ctx context.Context, today Date, q ChangesQuery) (dto.ChangesResponse, error) {
	out := dto.ChangesResponse{Items: []dto.Change{}}
	limit, offset, err := paging(q.Limit, q.Offset)
	if err != nil {
		return out, err
	}
	thr := q.Threshold
	if thr == 0 {
		thr = ChangeThreshold
	}
	if thr < 1 || thr > 100 {
		return out, &ValidationError{Errors: []dto.FieldError{{Path: "threshold", Code: "invalid"}}}
	}
	out.Threshold = thr
	from, to, err := r.window(ctx, today, q.Period, true)
	if err != nil {
		return out, err
	}
	depts, err := r.departments(ctx)
	if err != nil {
		return out, err
	}
	if q.Department != nil {
		if err := depts.check("departmentId", *q.Department); err != nil {
			return out, err
		}
	}
	cat, err := r.characteristics(ctx)
	if err != nil {
		return out, err
	}
	chars, err := r.selection(ctx, cat, q.CharacteristicIDs, from, to)
	if err != nil {
		return out, err
	}
	a := args{cidsOf(chars), from.String(), to.String(), thr}
	// Pairs are filtered, counted and paged in SQL (count(*) OVER ()), so only one
	// page crosses the wire (bench: "all" yields ~10^5 pairs).
	base := `
		WITH x AS (
		  SELECT t.user_id, s.characteristic_id AS cid, s.score, t.completed_at,
		         lag(s.score) OVER w AS ps, lag(t.completed_at) OVER w AS pa
		    FROM users u JOIN attempts t ON t.user_id = u.id JOIN attempt_scores s ON s.attempt_id = t.id
		   WHERE ` + popSQL + popFilter{Dept: q.Department}.cond(&a) + ` AND s.characteristic_id = ANY($1::bigint[])
		     AND t.completed_at < ` + endOf("$3") + `
		  WINDOW w AS (PARTITION BY t.user_id, s.characteristic_id ORDER BY t.completed_at, t.id)),
		c AS (
		  SELECT * FROM x
		   WHERE x.ps IS NOT NULL AND x.completed_at >= ` + startOf("$2") + ` AND abs(x.score - x.ps) >= $4::numeric)`
	lim, off := a.add(limit), a.add(offset)
	rows, err := r.pool.Query(ctx, base+`
		SELECT u.id, u.full_name, u.department_id, c.cid, c.ps::float8, c.pa, c.score::float8, c.completed_at,
		       count(*) OVER ()::int
		  FROM c JOIN users u ON u.id = c.user_id
		 ORDER BY abs(c.score - c.ps) DESC, c.completed_at DESC, u.id, c.cid
		 LIMIT `+lim+` OFFSET `+off, a...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c dto.Change
		var dk *int64
		if err := rows.Scan(&c.User.ID, &c.User.FullName, &dk, &c.CharacteristicID, &c.From.Score, &c.From.At,
			&c.To.Score, &c.To.At, &out.Total); err != nil {
			return out, err
		}
		c.Department = depts.ref(dk)
		c.Delta = round2(c.To.Score - c.From.Score)
		out.Items = append(out.Items, c)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	if len(out.Items) == 0 && offset > 0 { // past the end: the window count is unavailable
		err = r.pool.QueryRow(ctx, base+` SELECT count(*)::int FROM c`, a[:len(a)-2]...).Scan(&out.Total)
	}
	return out, err
}
