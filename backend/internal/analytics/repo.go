package analytics

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"bigroup-backend/internal/httpapi/dto"
)

// New returns the production Repo: on-demand SQL event sweep, no internal cache.
// The 60 s response cache lives in the HTTP layer (it needs the caller's role class),
// so rdb is accepted for the frozen signature but unused here.
func New(pool *pgxpool.Pool, rdb *redis.Client) Repo {
	_ = rdb
	return &repo{pool: pool}
}

type repo struct{ pool *pgxpool.Pool }

// ---------- characteristics and departments ----------

type catalog struct {
	ordered []dto.CharacteristicRef // core first, then id
	byID    map[int64]dto.CharacteristicRef
}

func (r *repo) characteristics(ctx context.Context) (catalog, error) {
	c := catalog{byID: map[int64]dto.CharacteristicRef{}}
	rows, err := r.pool.Query(ctx, `SELECT id, name, name_i18n, is_default FROM characteristics ORDER BY NOT is_default, id`)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var ch dto.CharacteristicRef
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.NameI18n, &ch.Core); err != nil {
			return c, err
		}
		c.ordered = append(c.ordered, ch)
		c.byID[ch.ID] = ch
	}
	return c, rows.Err()
}

func (c catalog) core() []dto.CharacteristicRef {
	var out []dto.CharacteristicRef
	for _, ch := range c.ordered {
		if ch.Core {
			out = append(out, ch)
		}
	}
	return out
}

// pick resolves requested ids (deduplicated, order preserved). Unknown ids or more
// than MaxCharacteristics yield a 422 on `characteristics`.
func (c catalog) pick(ids []int64) ([]dto.CharacteristicRef, error) {
	seen := map[int64]bool{}
	var out []dto.CharacteristicRef
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		ch, ok := c.byID[id]
		if !ok {
			return nil, &ValidationError{Errors: []dto.FieldError{{Path: "characteristics", Code: "unknown"}}}
		}
		out = append(out, ch)
	}
	if len(out) > MaxCharacteristics {
		return nil, &ValidationError{Errors: []dto.FieldError{{Path: "characteristics", Code: "too_many"}}}
	}
	return out, nil
}

func cidsOf(chars []dto.CharacteristicRef) []int64 {
	out := make([]int64, len(chars))
	for i, c := range chars {
		out[i] = c.ID
	}
	return out
}

type deptMap map[int64]dto.AnalyticsDepartmentRef

func (r *repo) departments(ctx context.Context) (deptMap, error) {
	m := deptMap{}
	rows, err := r.pool.Query(ctx, `SELECT id, name, active FROM departments`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d dto.AnalyticsDepartmentRef
		if err := rows.Scan(&d.ID, &d.Name, &d.Active); err != nil {
			return nil, err
		}
		m[d.ID] = d
	}
	return m, rows.Err()
}

// ref returns the department of key dk (noDept or a NULL id = nil).
func (m deptMap) ref(dk *int64) *dto.AnalyticsDepartmentRef {
	if dk == nil || *dk == noDept {
		return nil
	}
	d, ok := m[*dk]
	if !ok {
		return nil
	}
	return &d
}

// check validates department selections against the table (422 `unknown` on path).
func (m deptMap) check(path string, sels ...DepartmentSel) error {
	for _, s := range sels {
		if s.None {
			continue
		}
		if _, ok := m[s.ID]; !ok {
			return &ValidationError{Errors: []dto.FieldError{{Path: path, Code: "unknown"}}}
		}
	}
	return nil
}

func (s DepartmentSel) key() int64 {
	if s.None {
		return noDept
	}
	return s.ID
}

// ---------- window ----------

// window validates and resolves the period. PeriodAll starts at the earliest
// population measurement day (whole company, day <= today).
func (r *repo) window(ctx context.Context, today Date, q PeriodQuery, allowCustom bool) (Date, Date, error) {
	if err := checkPeriod(today, q, allowCustom); err != nil {
		return Date{}, Date{}, err
	}
	var earliest *Date
	if q.Preset == PeriodAll {
		var t *time.Time
		err := r.pool.QueryRow(ctx, `
			SELECT min(t.completed_at) FROM attempts t JOIN users u ON u.id = t.user_id
			 WHERE `+popSQL+` AND t.completed_at < `+endOf("$1"), today.String()).Scan(&t)
		if err != nil {
			return Date{}, Date{}, err
		}
		if t != nil {
			d := DateOf(*t)
			earliest = &d
		}
	}
	from, to := ResolveWindow(today, q, earliest)
	return from, to, nil
}

// ---------- coverage and default selection ----------

type coverage struct{ employees, attempts int }

// coverageIn counts, per characteristic, distinct population employees and attempts
// with a measurement day in [from, to].
func (r *repo) coverageIn(ctx context.Context, from, to Date, f popFilter) (map[int64]coverage, error) {
	a := args{from.String(), to.String()}
	// Two-level hash aggregation (per user, then per characteristic) instead of
	// count(DISTINCT), which sorts.
	q := `SELECT cid, count(*)::int, sum(n)::int FROM (
	        SELECT s.characteristic_id AS cid, t.user_id, count(*) AS n
	          FROM users u JOIN attempts t ON t.user_id = u.id JOIN attempt_scores s ON s.attempt_id = t.id
	         WHERE ` + popSQL + f.cond(&a) + `
	           AND t.completed_at >= ` + startOf("$1") + ` AND t.completed_at < ` + endOf("$2") + `
	         GROUP BY 1, 2) x
	       GROUP BY 1`
	rows, err := r.pool.Query(ctx, q, a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]coverage{}
	for rows.Next() {
		var id int64
		var c coverage
		if err := rows.Scan(&id, &c.employees, &c.attempts); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}

// defaultSelection: every core characteristic plus up to 3 non-core ones ranked by
// coverage (employees desc, attempts desc, id asc) with employees >= MinSample.
func defaultSelection(cat catalog, cov map[int64]coverage) []dto.CharacteristicRef {
	out := cat.core()
	var extra []dto.CharacteristicRef
	for _, ch := range cat.ordered {
		if !ch.Core && cov[ch.ID].employees >= MinSample {
			extra = append(extra, ch)
		}
	}
	sort.SliceStable(extra, func(i, j int) bool {
		ci, cj := cov[extra[i].ID], cov[extra[j].ID]
		if ci.employees != cj.employees {
			return ci.employees > cj.employees
		}
		if ci.attempts != cj.attempts {
			return ci.attempts > cj.attempts
		}
		return extra[i].ID < extra[j].ID
	})
	if len(extra) > 3 {
		extra = extra[:3]
	}
	return append(out, extra...)
}

// selection resolves requested ids, or the default selection when none were given.
func (r *repo) selection(ctx context.Context, cat catalog, ids []int64, from, to Date) ([]dto.CharacteristicRef, error) {
	if len(ids) > 0 {
		return cat.pick(ids)
	}
	cov, err := r.coverageIn(ctx, from, to, popFilter{})
	if err != nil {
		return nil, err
	}
	return defaultSelection(cat, cov), nil
}

// ---------- trend ----------

func (r *repo) DashboardTrend(ctx context.Context, today Date, period PeriodQuery) (dto.TrendResponse, error) {
	from, to, err := r.window(ctx, today, period, false)
	if err != nil {
		return dto.TrendResponse{}, err
	}
	cat, err := r.characteristics(ctx)
	if err != nil {
		return dto.TrendResponse{}, err
	}
	return r.trend(ctx, from, to, cat.core())
}

func (r *repo) AnalyticsTrend(ctx context.Context, today Date, q TrendQuery) (dto.AnalyticsTrendResponse, error) {
	var out dto.AnalyticsTrendResponse
	from, to, err := r.window(ctx, today, q.Period, true)
	if err != nil {
		return out, err
	}
	cat, err := r.characteristics(ctx)
	if err != nil {
		return out, err
	}
	var cov map[int64]coverage
	if len(q.CharacteristicIDs) > 0 {
		chars, err := cat.pick(q.CharacteristicIDs)
		if err != nil {
			return out, err
		}
		// Coverage and the series are independent: fetch them concurrently.
		err = parallel(
			func() (err error) { cov, err = r.coverageIn(ctx, from, to, popFilter{}); return err },
			func() (err error) { out.TrendResponse, err = r.trend(ctx, from, to, chars); return err },
		)
		if err != nil {
			return out, err
		}
	} else {
		if cov, err = r.coverageIn(ctx, from, to, popFilter{}); err != nil {
			return out, err
		}
		if out.TrendResponse, err = r.trend(ctx, from, to, defaultSelection(cat, cov)); err != nil {
			return out, err
		}
	}
	def := defaultSelection(cat, cov)
	out.DefaultIDs = cidsOf(def)
	out.Characteristics = make([]dto.CharacteristicCoverage, 0, len(cat.ordered))
	for _, ch := range cat.ordered {
		c := cov[ch.ID]
		out.Characteristics = append(out.Characteristics, dto.CharacteristicCoverage{
			CharacteristicRef: ch, Employees: c.employees, Attempts: c.attempts, Eligible: c.employees >= MinSample,
		})
	}
	return out, nil
}

// trend builds the company series for chars over [from, to] (unredacted admin form).
func (r *repo) trend(ctx context.Context, from, to Date, chars []dto.CharacteristicRef) (dto.TrendResponse, error) {
	ends := BucketEnds(from, to, Granularity(from, to))
	out := dto.TrendResponse{
		Period: PeriodInfo(from, to), MinSample: MinSample,
		Points: dateStrings(ends), Series: make([]dto.TrendSeries, 0, len(chars)),
	}
	cids := cidsOf(chars)
	var runs map[groupKey][]runRow
	err := parallel(
		func() (err error) { runs, err = r.sweep(ctx, cids, to, popFilter{}, false); return err },
		func() error {
			a := args{from.String(), to.String(), cids}
			return r.pool.QueryRow(ctx, `
				SELECT (SELECT count(*) FROM users u WHERE `+popSQL+`)::int,
				       (SELECT count(*) FROM users u WHERE `+popSQL+` AND EXISTS (
				          SELECT 1 FROM attempts t JOIN attempt_scores s ON s.attempt_id = t.id
				           WHERE t.user_id = u.id AND s.characteristic_id = ANY($3::bigint[])
				             AND t.completed_at < `+endOf("$2")+`))::int,
				       (SELECT count(*) FROM users u JOIN attempts t ON t.user_id = u.id
				         WHERE `+popSQL+` AND t.completed_at >= `+startOf("$1")+` AND t.completed_at < `+endOf("$2")+`)::int`,
				a...).Scan(&out.Summary.Employees, &out.Summary.MeasuredEmployees, &out.Summary.AttemptsInPeriod)
		},
	)
	if err != nil {
		return out, err
	}
	for _, ch := range chars {
		s := dto.TrendSeries{Characteristic: ch, Values: make([]dto.ValuePoint, len(ends))}
		for i, g := range sample(runs[groupKey{companyKey, ch.ID}], ends) {
			s.Values[i] = g.point(false)
		}
		out.Series = append(out.Series, s)
	}
	return out, nil
}

// ---------- departments ----------

func (r *repo) Departments(ctx context.Context, today Date, q DepartmentsQuery) (dto.DepartmentsResponse, error) {
	var out dto.DepartmentsResponse
	from, to, err := r.window(ctx, today, q.Period, true)
	if err != nil {
		return out, err
	}
	if len(q.Departments) == 0 || len(q.Departments) > MaxDepartments {
		return out, &ValidationError{Errors: []dto.FieldError{{Path: "departments", Code: "count"}}}
	}
	depts, err := r.departments(ctx)
	if err != nil {
		return out, err
	}
	if err := depts.check("departments", q.Departments...); err != nil {
		return out, err
	}
	cat, err := r.characteristics(ctx)
	if err != nil {
		return out, err
	}
	chars, err := r.selection(ctx, cat, q.CharacteristicIDs, from, to)
	if err != nil {
		return out, err
	}
	// Deduplicate the compared set, order preserved.
	var sels []DepartmentSel
	seen := map[int64]bool{}
	for _, s := range q.Departments {
		if !seen[s.key()] {
			seen[s.key()] = true
			sels = append(sels, s)
		}
	}
	ends := BucketEnds(from, to, Granularity(from, to))
	out.Period, out.Points = PeriodInfo(from, to), dateStrings(ends)
	cids := cidsOf(chars)
	// One measurement scan (whole population, keyed by department) feeds both the
	// company sweep and the per-department sweep; headcounts and attempts in the
	// window per (department, characteristic) are fetched concurrently.
	var ms []measurement
	heads := map[int64]int{}
	inPeriod := map[groupKey]int{}
	err = parallel(
		func() (err error) { ms, err = r.measurements(ctx, cids, to, popFilter{}, true); return err },
		func() error {
			ha := args{}
			rows, err := r.pool.Query(ctx, `SELECT COALESCE(u.department_id, 0), count(*)::int FROM users u WHERE `+popSQL+popFilter{Depts: sels}.cond(&ha)+` GROUP BY 1`, ha...)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var dk int64
				var n int
				if err := rows.Scan(&dk, &n); err != nil {
					return err
				}
				heads[dk] = n
			}
			return rows.Err()
		},
		func() error {
			a := args{from.String(), to.String(), cids}
			rows, err := r.pool.Query(ctx, `
				SELECT COALESCE(u.department_id, 0), s.characteristic_id, count(*)::int
				  FROM users u JOIN attempts t ON t.user_id = u.id JOIN attempt_scores s ON s.attempt_id = t.id
				 WHERE `+popSQL+popFilter{Depts: sels}.cond(&a)+` AND s.characteristic_id = ANY($3::bigint[])
				   AND t.completed_at >= `+startOf("$1")+` AND t.completed_at < `+endOf("$2")+`
				 GROUP BY 1, 2`, a...)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var k groupKey
				var n int
				if err := rows.Scan(&k.dk, &k.cid, &n); err != nil {
					return err
				}
				inPeriod[k] = n
			}
			return rows.Err()
		},
	)
	if err != nil {
		return out, err
	}
	selected := map[int64]bool{}
	for _, s := range sels {
		selected[s.key()] = true
	}
	var deptMs []measurement
	for _, m := range ms {
		if selected[m.dk] {
			deptMs = append(deptMs, m)
		}
	}
	byDept := sweepRows(deptMs)
	for i := range ms {
		ms[i].dk = companyKey
	}
	company := sweepRows(ms)

	out.Blocks = make([]dto.DepartmentsBlock, 0, len(chars))
	for _, ch := range chars {
		b := dto.DepartmentsBlock{Characteristic: ch, Company: dto.CompanySeries{Values: make([]dto.ValuePoint, len(ends))}}
		cs := sample(company[groupKey{companyKey, ch.ID}], ends)
		for i, g := range cs {
			b.Company.Values[i] = g.point(false)
		}
		companyNow := cs[len(cs)-1].point(false)
		b.Departments = make([]dto.DepartmentSeries, 0, len(sels))
		for _, s := range sels {
			k := s.key()
			ds := dto.DepartmentSeries{
				Department: depts.ref(&k), Employees: heads[k], AttemptsInPeriod: inPeriod[groupKey{k, ch.ID}],
				Values: make([]dto.ValuePoint, len(ends)),
			}
			dsamp := sample(byDept[groupKey{k, ch.ID}], ends)
			for i, g := range dsamp {
				ds.Values[i] = g.point(true)
			}
			ds.Current.ValuePoint = dsamp[len(dsamp)-1].point(true)
			if ds.Current.Value != nil && companyNow.Value != nil && !companyNow.Insufficient {
				v := round2(*ds.Current.Value - *companyNow.Value)
				ds.Current.VsCompany = &v
			}
			b.Departments = append(b.Departments, ds)
		}
		out.Blocks = append(out.Blocks, b)
	}
	return out, nil
}
