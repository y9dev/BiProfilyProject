package analytics

import (
	"context"
	"maps"
	"sort"
	"time"

	"bigroup-backend/internal/assessments"
	"bigroup-backend/internal/httpapi/dto"
)

// Limits of the team status payload (plan sec. 3).
const (
	maxTests     = 20
	maxAttention = 50
)

// member is a population member with their attempt stats as of a day.
type member struct {
	id        int64
	fullName  string
	dept      *int64
	createdAt time.Time
	last      *time.Time
	attempts  int
}

// members lists the population (optionally filtered) with attempt count and last
// attempt among attempts whose day <= asOf.
func (r *repo) members(ctx context.Context, asOf Date, f popFilter) ([]member, error) {
	a := args{asOf.String()}
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.full_name, u.department_id, u.created_at, max(t.completed_at), count(t.id)::int
		  FROM users u LEFT JOIN attempts t ON t.user_id = u.id AND t.completed_at < `+endOf("$1")+`
		 WHERE `+popSQL+f.cond(&a)+`
		 GROUP BY u.id ORDER BY u.id`, a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.fullName, &m.dept, &m.createdAt, &m.last, &m.attempts); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// testCompletion lists employee-visible assessments (newest first, capped) with the
// attempts population members made on them with from <= day <= to (from nil = ever).
// Rate is completed / employees; null when employees is 0 or below minRate.
func (r *repo) testCompletion(ctx context.Context, from *Date, to Date, f popFilter, employees, minRate int) ([]dto.TestCompletion, error) {
	a := args{to.String()}
	lower := ""
	if from != nil {
		lower = ` AND t.completed_at >= ` + startOf(a.add(from.String()))
	}
	cond := f.cond(&a)
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.title,
		       (SELECT count(*) FROM attempts t JOIN users u ON u.id = t.user_id
		         WHERE t.assessment_id = a.id AND `+popSQL+cond+`
		           AND t.completed_at < `+endOf("$1")+lower+`)::int
		  FROM assessments a
		 WHERE `+assessments.VisibleSQL+`
		 ORDER BY a.created_at DESC, a.id DESC LIMIT `+itoa(maxTests), a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dto.TestCompletion{}
	for rows.Next() {
		tc := dto.TestCompletion{Employees: employees}
		if err := rows.Scan(&tc.ID, &tc.Title, &tc.Completed); err != nil {
			return nil, err
		}
		if employees >= minRate {
			tc.Rate = ratio(tc.Completed, employees)
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

type drop struct {
	cid   int64
	delta float64
	at    time.Time
}

// drops returns, per population member, the worst characteristic whose last two
// measurements (by completed_at, attempt id; no day dedupe) fell by >= ChangeThreshold.
// The scan over all scores is split into user-id partitions run concurrently (bench).
func (r *repo) drops(ctx context.Context, today Date) (map[int64]drop, error) {
	parts := make([]map[int64]drop, scanParts)
	fns := make([]func() error, scanParts)
	for i := range fns {
		fns[i] = func() error {
			parts[i] = map[int64]drop{}
			rows, err := r.pool.Query(ctx, `
				WITH x AS (
				  SELECT t.user_id, s.characteristic_id AS cid, s.score, t.completed_at,
				         row_number() OVER (PARTITION BY t.user_id, s.characteristic_id ORDER BY t.completed_at DESC, t.id DESC) AS rn
				    FROM users u JOIN attempts t ON t.user_id = u.id JOIN attempt_scores s ON s.attempt_id = t.id
				   WHERE `+popSQL+` AND u.id % $3 = $4 AND t.completed_at < `+endOf("$1")+`)
				SELECT a.user_id, a.cid, (a.score - b.score)::float8, a.completed_at
				  FROM x a JOIN x b ON b.user_id = a.user_id AND b.cid = a.cid AND b.rn = 2
				 WHERE a.rn = 1 AND a.score - b.score <= -$2::numeric
				 ORDER BY a.user_id, a.score - b.score, a.cid`, today.String(), ChangeThreshold, scanParts, i)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var uid int64
				var d drop
				if err := rows.Scan(&uid, &d.cid, &d.delta, &d.at); err != nil {
					return err
				}
				if _, ok := parts[i][uid]; !ok { // first row per user = worst delta (ties: lowest id)
					d.delta = round2(d.delta)
					parts[i][uid] = d
				}
			}
			return rows.Err()
		}
	}
	if err := parallel(fns...); err != nil {
		return nil, err
	}
	out := map[int64]drop{}
	for _, p := range parts {
		maps.Copy(out, p)
	}
	return out, nil
}

func (r *repo) TeamStatus(ctx context.Context, today Date) (dto.TeamStatus, error) {
	out := dto.TeamStatus{
		AsOf: today.String(), InactiveDays: InactiveDays, DropThreshold: ChangeThreshold,
		Tests: []dto.TestCompletion{}, Attention: dto.Attention{Items: []dto.AttentionItem{}},
	}
	var (
		ms    []member
		depts deptMap
		drops map[int64]drop
	)
	// Independent reads run concurrently; test rates are filled in once |P| is known.
	err := parallel(
		func() (err error) { ms, err = r.members(ctx, today, popFilter{}); return err },
		func() (err error) { depts, err = r.departments(ctx); return err },
		func() (err error) { drops, err = r.drops(ctx, today); return err },
		func() (err error) { out.Tests, err = r.testCompletion(ctx, nil, today, popFilter{}, 0, 1); return err },
	)
	if err != nil {
		return out, err
	}
	type ranked struct {
		item      dto.AttentionItem
		class     int // 0 drop, 1 inactive, 2 no_first_test
		magnitude float64
		createdAt time.Time
		id        int64
	}
	var att []ranked
	k := &out.KPIs
	k.Employees = len(ms)
	for _, m := range ms {
		it := dto.AttentionItem{User: dto.UserRef{ID: m.id, FullName: m.fullName}, Department: depts.ref(m.dept), LastAttemptAt: m.last, Reasons: []dto.Reason{}}
		rk := ranked{id: m.id, createdAt: m.createdAt, class: -1}
		if m.attempts == 0 {
			it.Reasons = append(it.Reasons, dto.Reason{Code: "no_first_test"})
			out.Attention.Counts.NoFirstTest++
			rk.class = 2
		} else {
			k.Participated++
			days := DateOf(*m.last).DaysUntil(today)
			if d, ok := drops[m.id]; ok {
				cid, delta := d.cid, d.delta
				it.Reasons = append(it.Reasons, dto.Reason{Code: "drop", CharacteristicID: &cid, Delta: &delta, At: DateOf(d.at).String()})
				out.Attention.Counts.Drop++
				rk.class, rk.magnitude = 0, -delta
			}
			if days >= InactiveDays {
				k.Inactive++
				dd := days
				it.Reasons = append(it.Reasons, dto.Reason{Code: "inactive", Days: &dd})
				out.Attention.Counts.Inactive++
				if rk.class < 0 {
					rk.class, rk.magnitude = 1, float64(days)
				}
			} else {
				k.ActiveRecently++
			}
		}
		if rk.class >= 0 {
			rk.item = it
			att = append(att, rk)
		}
	}
	k.NoAttempts = k.Employees - k.Participated
	k.ParticipationRate = ratio(k.Participated, k.Employees)

	sort.Slice(att, func(i, j int) bool {
		a, b := att[i], att[j]
		if a.class != b.class {
			return a.class < b.class
		}
		if a.class == 2 {
			if !a.createdAt.Equal(b.createdAt) {
				return a.createdAt.Before(b.createdAt)
			}
		} else if a.magnitude != b.magnitude {
			return a.magnitude > b.magnitude
		}
		return a.id < b.id
	})
	out.Attention.Total = len(att)
	for i := 0; i < len(att) && i < maxAttention; i++ {
		out.Attention.Items = append(out.Attention.Items, att[i].item)
	}

	for i := range out.Tests {
		out.Tests[i].Employees = k.Employees
		out.Tests[i].Rate = ratio(out.Tests[i].Completed, k.Employees)
	}
	return out, nil
}
