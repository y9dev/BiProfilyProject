// Package analytics computes company-level aggregates for the manager/admin
// dashboard and the admin analytics page (plan .omc/plans/manager-admin-analytics.md).
//
// FROZEN CONTRACT: the Repo interface, the query structs and the error types in this
// file are fixed; any signature change goes back through the contract owner.
// New (repo.go) returns the SQL event-sweep implementation. The 60 s response cache
// lives in the HTTP layer (httpapi/analytics.go), which codes against this interface only.
//
// Rules for every implementation:
//   - `today` is one Asia/Almaty date computed once per request by the caller and
//     passed explicitly; no SQL now()/current_date in business logic.
//   - Population: role='employee' AND status='active' at request time.
//   - Results are the unredacted (admin) form; the HTTP layer nulls values below
//     MinSample for managers.
//   - Unknown characteristic or department ids yield *ValidationError (422);
//     a missing employee yields ErrNotFound (404).
//   - ctx carries the QueryTimeout deadline; on expiry return an error wrapping
//     context.DeadlineExceeded (or the pgx 57014 error) so the handler can answer 503.
package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bigroup-backend/internal/httpapi/dto"
)

var (
	// ErrNotFound: the employee view subject does not exist or is not role=employee.
	ErrNotFound = errors.New("analytics: not found")
)

// ValidationError carries 422 field errors (e.g. {characteristics, unknown}).
type ValidationError struct{ Errors []dto.FieldError }

func (e *ValidationError) Error() string { return "analytics: invalid query" }

// Date is an Asia/Almaty calendar day.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// String formats the date as YYYY-MM-DD (the wire format of PeriodInfo, asOf, points).
func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day) }

// Period presets accepted in PeriodQuery.Preset.
const (
	Period7d     = "7d"
	Period30d    = "30d"
	Period90d    = "90d"
	Period12m    = "12m"
	PeriodAll    = "all"
	PeriodCustom = "custom"
)

// Employee list data filters (ListQuery.Data) and AnalyticsEmployee.DataState values.
const (
	DataAll    = "all"
	DataOK     = "ok"
	DataNoData = "no_data"
	DataStale  = "stale"
)

// PeriodQuery is the syntactically validated period request. From/To are set only
// with PeriodCustom. The repo resolves the window (PeriodAll needs the earliest
// population measurement) and returns it as dto.PeriodInfo.
type PeriodQuery struct {
	Preset string
	From   Date
	To     Date
}

// DepartmentSel selects one department, or "No department" when None is true.
type DepartmentSel struct {
	ID   int64
	None bool
}

// TrendQuery: /api/analytics/trend. Empty CharacteristicIDs = default selection
// (core + up to 3 non-core by coverage). Order is preserved, duplicates removed by the caller.
type TrendQuery struct {
	Period            PeriodQuery
	CharacteristicIDs []int64
}

// ParticipationQuery: /api/analytics/participation. Department nil = whole company.
type ParticipationQuery struct {
	Period     PeriodQuery
	Department *DepartmentSel
}

// DepartmentsQuery: /api/analytics/departments. Departments has 1..MaxDepartments entries.
type DepartmentsQuery struct {
	Period            PeriodQuery
	CharacteristicIDs []int64
	Departments       []DepartmentSel
}

// ListQuery: /api/analytics/employees. Data is DataAll | DataNoData | DataStale.
type ListQuery struct {
	Search     string
	Department *DepartmentSel
	Data       string
	Limit      int
	Offset     int
}

// RankingQuery: /api/analytics/ranking. Limit/Offset apply per characteristic block.
type RankingQuery struct {
	Period            PeriodQuery
	CharacteristicIDs []int64
	Department        *DepartmentSel
	Limit             int
	Offset            int
}

// ChangesQuery: /api/analytics/changes. Threshold is 1..100 (default ChangeThreshold).
type ChangesQuery struct {
	Period            PeriodQuery
	CharacteristicIDs []int64
	Department        *DepartmentSel
	Threshold         int
	Limit             int
	Offset            int
}

// Repo is the analytics engine. Every method takes ctx and an explicit `today`.
type Repo interface {
	// DashboardTrend: core characteristics only; period is a preset (never custom).
	DashboardTrend(ctx context.Context, today Date, period PeriodQuery) (dto.TrendResponse, error)
	// TeamStatus: KPIs, per-test completion and the attention list as of today.
	TeamStatus(ctx context.Context, today Date) (dto.TeamStatus, error)
	// AnalyticsTrend: selected (or default) series plus the characteristic catalog.
	AnalyticsTrend(ctx context.Context, today Date, q TrendQuery) (dto.AnalyticsTrendResponse, error)
	// Participation: window-bound participation and per-test completion.
	Participation(ctx context.Context, today Date, q ParticipationQuery) (dto.ParticipationResponse, error)
	// Departments: department comparison, one block per characteristic.
	Departments(ctx context.Context, today Date, q DepartmentsQuery) (dto.DepartmentsResponse, error)
	// Employees: active employees with their data state.
	Employees(ctx context.Context, today Date, q ListQuery) (dto.AnalyticsEmployeeList, error)
	// Employee: one employee (blocked allowed), every characteristic they have, as of the period end.
	Employee(ctx context.Context, today Date, userID int64, period PeriodQuery) (dto.EmployeeAnalytics, error)
	// Ranking: one block per characteristic, as of the period end.
	Ranking(ctx context.Context, today Date, q RankingQuery) (dto.RankingResponse, error)
	// Changes: significant consecutive-measurement changes within the period.
	Changes(ctx context.Context, today Date, q ChangesQuery) (dto.ChangesResponse, error)
}

// New (repo.go) returns the production Repo: the SQL event sweep implementation.
