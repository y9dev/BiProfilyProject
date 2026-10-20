package analytics

import (
	"context"
	"sort"
	"strings"

	"bigroup-backend/internal/httpapi/dto"
)

// Participation: window-bound participation per company and department, and per-test
// completion. With a department filter, company and tests cover that department only.
// Every rate here is null below MinSample employees.
func (r *repo) Participation(ctx context.Context, today Date, q ParticipationQuery) (dto.ParticipationResponse, error) {
	out := dto.ParticipationResponse{Departments: []dto.ParticipationDepartment{}, Tests: []dto.TestCompletion{}}
	from, to, err := r.window(ctx, today, q.Period, true)
	if err != nil {
		return out, err
	}
	depts, err := r.departments(ctx)
	if err != nil {
		return out, err
	}
	f := popFilter{Dept: q.Department}
	if q.Department != nil {
		if err := depts.check("departmentId", *q.Department); err != nil {
			return out, err
		}
	}
	out.Period = PeriodInfo(from, to)

	a := args{from.String(), to.String()}
	rows, err := r.pool.Query(ctx, `
		SELECT u.department_id, count(*)::int, count(*) FILTER (WHERE x.n > 0)::int, COALESCE(sum(x.n), 0)::int
		  FROM users u
		  LEFT JOIN LATERAL (
		       SELECT count(*) AS n FROM attempts t
		        WHERE t.user_id = u.id AND t.completed_at >= `+startOf("$1")+` AND t.completed_at < `+endOf("$2")+`) x ON TRUE
		 WHERE `+popSQL+f.cond(&a)+`
		 GROUP BY 1`, a...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var dk *int64
		var d dto.ParticipationDepartment
		if err := rows.Scan(&dk, &d.Employees, &d.Participated, &d.Attempts); err != nil {
			return out, err
		}
		d.Department = depts.ref(dk)
		d.Insufficient = d.Employees < MinSample
		if !d.Insufficient {
			d.Rate = ratio(d.Participated, d.Employees)
		}
		out.Company.Employees += d.Employees
		out.Company.Participated += d.Participated
		out.Company.Attempts += d.Attempts
		out.Departments = append(out.Departments, d)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	if out.Company.Employees >= MinSample {
		out.Company.Rate = ratio(out.Company.Participated, out.Company.Employees)
	}
	// Departments by name; "No department" last.
	sort.Slice(out.Departments, func(i, j int) bool {
		a, b := out.Departments[i].Department, out.Departments[j].Department
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		if la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name); la != lb {
			return la < lb
		}
		return a.ID < b.ID
	})
	out.Tests, err = r.testCompletion(ctx, &from, to, f, out.Company.Employees, MinSample)
	return out, err
}
