package analytics

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"bigroup-backend/internal/httpapi/dto"
)

// Paging defaults and caps shared by list endpoints.
const (
	defaultLimit = 50
	maxLimit     = 200
)

// paging normalizes limit/offset (0 = default limit) and rejects out-of-range values.
func paging(limit, offset int) (int, int, error) {
	if limit < 0 || limit > maxLimit {
		return 0, 0, &ValidationError{Errors: []dto.FieldError{{Path: "limit", Code: "invalid"}}}
	}
	if offset < 0 {
		return 0, 0, &ValidationError{Errors: []dto.FieldError{{Path: "offset", Code: "invalid"}}}
	}
	if limit == 0 {
		limit = defaultLimit
	}
	return limit, offset, nil
}

func page[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := min(offset+limit, len(items))
	return items[offset:end]
}

// likeEscape escapes LIKE wildcards (backslash is Postgres' default escape).
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// dataState classifies an employee as of today: no attempts, stale (last attempt
// day <= today - InactiveDays) or ok.
func dataState(today Date, attempts int, last *time.Time) string {
	switch {
	case attempts == 0 || last == nil:
		return DataNoData
	case DateOf(*last).DaysUntil(today) >= InactiveDays:
		return DataStale
	}
	return DataOK
}

func (r *repo) Employees(ctx context.Context, today Date, q ListQuery) (dto.AnalyticsEmployeeList, error) {
	out := dto.AnalyticsEmployeeList{Items: []dto.AnalyticsEmployee{}}
	limit, offset, err := paging(q.Limit, q.Offset)
	if err != nil {
		return out, err
	}
	data := q.Data
	switch data {
	case "":
		data = DataAll
	case DataAll, DataNoData, DataStale:
	default:
		return out, &ValidationError{Errors: []dto.FieldError{{Path: "data", Code: "invalid"}}}
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
	a := args{today.String()}
	cond := popFilter{Dept: q.Department}.cond(&a)
	if s := strings.TrimSpace(q.Search); s != "" {
		p := a.add("%" + likeEscape(s) + "%")
		cond += ` AND (u.full_name ILIKE ` + p + ` OR u.login::text ILIKE ` + p + `)`
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.full_name, u.job_title, u.department_id, count(t.id)::int, max(t.completed_at)
		  FROM users u LEFT JOIN attempts t ON t.user_id = u.id AND t.completed_at < `+endOf("$1")+`
		 WHERE `+popSQL+cond+`
		 GROUP BY u.id ORDER BY lower(u.full_name), u.id`, a...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var all []dto.AnalyticsEmployee
	for rows.Next() {
		var e dto.AnalyticsEmployee
		var dk *int64
		if err := rows.Scan(&e.ID, &e.FullName, &e.JobTitle, &dk, &e.Attempts, &e.LastAttemptAt); err != nil {
			return out, err
		}
		e.Department = depts.ref(dk)
		e.DataState = dataState(today, e.Attempts, e.LastAttemptAt)
		if data == DataAll || e.DataState == data {
			all = append(all, e)
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	out.Total = len(all)
	out.Items = page(all, limit, offset)
	return out, nil
}

// Employee: one employee (blocked allowed), every characteristic measured up to the
// period end `to`, with company and department comparisons as of `to` (comparison
// values withheld below MinSample) and the attempt history within the period.
func (r *repo) Employee(ctx context.Context, today Date, userID int64, period PeriodQuery) (dto.EmployeeAnalytics, error) {
	out := dto.EmployeeAnalytics{Current: []dto.EmployeeCurrent{}, History: []dto.TimelineEntry{}}
	var role string
	var dk *int64
	u := &out.User
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, u.full_name, u.job_title, u.department_id, u.status, u.role,
		       (SELECT max(t.completed_at) FROM attempts t WHERE t.user_id = u.id AND t.completed_at < `+endOf("$2")+`)
		  FROM users u WHERE u.id = $1`, userID, today.String()).
		Scan(&u.ID, &u.FullName, &u.JobTitle, &dk, &u.Status, &role, &u.LastAttemptAt)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && role != "employee") {
		return dto.EmployeeAnalytics{}, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	from, to, err := r.window(ctx, today, period, true)
	if err != nil {
		return out, err
	}
	if period.Preset == PeriodAll {
		// Also cover the subject's own history (a blocked employee is outside P).
		var first *time.Time
		if err := r.pool.QueryRow(ctx, `SELECT min(completed_at) FROM attempts WHERE user_id = $1`, userID).Scan(&first); err != nil {
			return out, err
		}
		if first != nil {
			if d := DateOf(*first); d.Before(from) {
				from, to = ResolveWindow(today, period, &d)
			}
		}
	}
	depts, err := r.departments(ctx)
	if err != nil {
		return out, err
	}
	u.Department = depts.ref(dk)
	out.AsOf = to.String()
	cat, err := r.characteristics(ctx)
	if err != nil {
		return out, err
	}

	type latest struct {
		score float64
		at    time.Time
	}
	cur := map[int64]latest{}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (s.characteristic_id) s.characteristic_id, s.score::float8, t.completed_at
		  FROM attempts t JOIN attempt_scores s ON s.attempt_id = t.id
		 WHERE t.user_id = $1 AND t.completed_at < `+endOf("$2")+`
		 ORDER BY s.characteristic_id, t.completed_at DESC, t.id DESC`, userID, to.String())
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var cid int64
		var l latest
		if err := rows.Scan(&cid, &l.score, &l.at); err != nil {
			rows.Close()
			return out, err
		}
		cur[cid] = l
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	var chars []dto.CharacteristicRef
	for _, ch := range cat.ordered {
		if _, ok := cur[ch.ID]; ok {
			chars = append(chars, ch)
		}
	}
	if len(chars) > 0 {
		cids := cidsOf(chars)
		ends := []Date{to}
		company, err := r.sweep(ctx, cids, to, popFilter{}, false)
		if err != nil {
			return out, err
		}
		sel := DepartmentSel{None: dk == nil}
		if dk != nil {
			sel.ID = *dk
		}
		dept, err := r.sweep(ctx, cids, to, popFilter{Dept: &sel}, false)
		if err != nil {
			return out, err
		}
		for _, ch := range chars {
			l := cur[ch.ID]
			out.Current = append(out.Current, dto.EmployeeCurrent{
				Characteristic: ch, Score: l.score, MeasuredAt: l.at,
				Company:    sample(company[groupKey{companyKey, ch.ID}], ends)[0].point(true),
				Department: sample(dept[groupKey{companyKey, ch.ID}], ends)[0].point(true),
			})
		}
	}

	out.History, err = r.history(ctx, userID, from, to)
	return out, err
}

// history is the subject's attempts with from <= day <= to, chronological, with scores
// (the TimelineEntry shape of /api/me/profile).
func (r *repo) history(ctx context.Context, userID int64, from, to Date) ([]dto.TimelineEntry, error) {
	out := []dto.TimelineEntry{}
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, a.title, t.completed_at, c.id, c.name, c.name_i18n, s.score::float8
		  FROM attempts t
		  JOIN assessments a ON a.id = t.assessment_id
		  LEFT JOIN attempt_scores s ON s.attempt_id = t.id
		  LEFT JOIN characteristics c ON c.id = s.characteristic_id
		 WHERE t.user_id = $1 AND t.completed_at >= `+startOf("$2")+` AND t.completed_at < `+endOf("$3")+`
		 ORDER BY t.completed_at, t.id, c.id`, userID, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e dto.TimelineEntry
		var cid *int64
		var name *string
		var i18n map[string]string
		var score *float64
		if err := rows.Scan(&e.AttemptID, &e.AssessmentTitle, &e.CompletedAt, &cid, &name, &i18n, &score); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].AttemptID != e.AttemptID {
			e.Scores = []dto.Score{}
			out = append(out, e)
		}
		if cid != nil {
			last := &out[len(out)-1]
			last.Scores = append(last.Scores, dto.Score{CharacteristicID: *cid, Name: *name, NameI18n: i18n, Score: *score})
		}
	}
	return out, rows.Err()
}
