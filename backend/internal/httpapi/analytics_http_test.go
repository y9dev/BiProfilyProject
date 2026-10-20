// HTTP tests for /api/dashboard/* and /api/analytics/*: authz matrix, query
// validation, error mapping, manager n<5 redaction, Redis cache and timeout.
// The analytics engine is replaced by a fake Repo (Deps.Analytics), so these
// tests exercise only the HTTP layer. They need Postgres (sessions, users, avatar
// gate) and Redis (sessions, cache) and skip like testDeps when unreachable.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"bigroup-backend/internal/admin"
	"bigroup-backend/internal/analytics"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/testutil"
	"bigroup-backend/internal/users"
)

const unknownID = 9999 // the fake treats this characteristic/department id as nonexistent

// fakeRepo is an in-memory analytics.Repo that counts calls.
type fakeRepo struct {
	calls     atomic.Int64
	block     bool  // wait for ctx expiry and return ctx.Err()
	err       error // returned by every method when set
	employees map[int64]string
	trend     dto.TrendResponse
	team      *dto.TeamStatus // TeamStatus result override
	gate      chan struct{}   // when set, calls wait for it to close (or ctx expiry)
	inflight  atomic.Int64    // calls currently waiting on gate
	maxFlight atomic.Int64    // peak of inflight
}

func (f *fakeRepo) enter(ctx context.Context) error {
	f.calls.Add(1)
	if f.gate != nil {
		n := f.inflight.Add(1)
		for m := f.maxFlight.Load(); n > m && !f.maxFlight.CompareAndSwap(m, n); m = f.maxFlight.Load() {
		}
		defer f.inflight.Add(-1)
		select {
		case <-f.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.err
}

func unknownIDs(chars []int64, depts ...analytics.DepartmentSel) error {
	var errs []dto.FieldError
	if slices.Contains(chars, unknownID) {
		errs = append(errs, dto.FieldError{Path: "characteristics", Code: "unknown"})
	}
	for _, d := range depts {
		if d.ID == unknownID {
			errs = append(errs, dto.FieldError{Path: "departments", Code: "unknown"})
		}
	}
	if errs != nil {
		return &analytics.ValidationError{Errors: errs}
	}
	return nil
}

func deptErr(d *analytics.DepartmentSel) error {
	if d != nil && d.ID == unknownID {
		return &analytics.ValidationError{Errors: []dto.FieldError{{Path: "departmentId", Code: "unknown"}}}
	}
	return nil
}

func (f *fakeRepo) DashboardTrend(ctx context.Context, _ analytics.Date, _ analytics.PeriodQuery) (dto.TrendResponse, error) {
	if err := f.enter(ctx); err != nil {
		return dto.TrendResponse{}, err
	}
	return f.trend, nil
}

func (f *fakeRepo) TeamStatus(ctx context.Context, today analytics.Date) (dto.TeamStatus, error) {
	if err := f.enter(ctx); err != nil {
		return dto.TeamStatus{}, err
	}
	if f.team != nil {
		return *f.team, nil
	}
	return dto.TeamStatus{AsOf: today.String(), InactiveDays: analytics.InactiveDays, DropThreshold: analytics.ChangeThreshold,
		Tests: []dto.TestCompletion{}, Attention: dto.Attention{Items: []dto.AttentionItem{}}}, nil
}

func (f *fakeRepo) AnalyticsTrend(ctx context.Context, _ analytics.Date, q analytics.TrendQuery) (dto.AnalyticsTrendResponse, error) {
	if err := f.enter(ctx); err != nil {
		return dto.AnalyticsTrendResponse{}, err
	}
	if err := unknownIDs(q.CharacteristicIDs); err != nil {
		return dto.AnalyticsTrendResponse{}, err
	}
	return dto.AnalyticsTrendResponse{TrendResponse: f.trend, DefaultIDs: []int64{}, Characteristics: []dto.CharacteristicCoverage{}}, nil
}

func (f *fakeRepo) Participation(ctx context.Context, _ analytics.Date, q analytics.ParticipationQuery) (dto.ParticipationResponse, error) {
	if err := f.enter(ctx); err != nil {
		return dto.ParticipationResponse{}, err
	}
	if err := deptErr(q.Department); err != nil {
		return dto.ParticipationResponse{}, err
	}
	return dto.ParticipationResponse{Departments: []dto.ParticipationDepartment{}, Tests: []dto.TestCompletion{}}, nil
}

func (f *fakeRepo) Departments(ctx context.Context, _ analytics.Date, q analytics.DepartmentsQuery) (dto.DepartmentsResponse, error) {
	if err := f.enter(ctx); err != nil {
		return dto.DepartmentsResponse{}, err
	}
	if err := unknownIDs(q.CharacteristicIDs, q.Departments...); err != nil {
		return dto.DepartmentsResponse{}, err
	}
	return dto.DepartmentsResponse{Points: []string{}, Blocks: []dto.DepartmentsBlock{}}, nil
}

func (f *fakeRepo) Employees(ctx context.Context, _ analytics.Date, q analytics.ListQuery) (dto.AnalyticsEmployeeList, error) {
	if err := f.enter(ctx); err != nil {
		return dto.AnalyticsEmployeeList{}, err
	}
	if err := deptErr(q.Department); err != nil {
		return dto.AnalyticsEmployeeList{}, err
	}
	return dto.AnalyticsEmployeeList{Items: []dto.AnalyticsEmployee{}}, nil
}

func (f *fakeRepo) Employee(ctx context.Context, _ analytics.Date, id int64, p analytics.PeriodQuery) (dto.EmployeeAnalytics, error) {
	if err := f.enter(ctx); err != nil {
		return dto.EmployeeAnalytics{}, err
	}
	status, ok := f.employees[id]
	if !ok {
		return dto.EmployeeAnalytics{}, fmt.Errorf("employee %d: %w", id, analytics.ErrNotFound)
	}
	return dto.EmployeeAnalytics{User: dto.AnalyticsUser{ID: id, Status: status},
		Current: []dto.EmployeeCurrent{}, History: []dto.TimelineEntry{}}, nil
}

func (f *fakeRepo) Ranking(ctx context.Context, _ analytics.Date, q analytics.RankingQuery) (dto.RankingResponse, error) {
	if err := f.enter(ctx); err != nil {
		return dto.RankingResponse{}, err
	}
	if err := unknownIDs(q.CharacteristicIDs); err != nil {
		return dto.RankingResponse{}, err
	}
	if err := deptErr(q.Department); err != nil {
		return dto.RankingResponse{}, err
	}
	return dto.RankingResponse{MinSample: analytics.MinSample, Blocks: []dto.RankingBlock{}}, nil
}

func (f *fakeRepo) Changes(ctx context.Context, _ analytics.Date, q analytics.ChangesQuery) (dto.ChangesResponse, error) {
	if err := f.enter(ctx); err != nil {
		return dto.ChangesResponse{}, err
	}
	if err := unknownIDs(q.CharacteristicIDs); err != nil {
		return dto.ChangesResponse{}, err
	}
	if err := deptErr(q.Department); err != nil {
		return dto.ChangesResponse{}, err
	}
	return dto.ChangesResponse{Threshold: q.Threshold, Items: []dto.Change{}}, nil
}

func fptr(v float64) *float64 { return &v }

// sampleTrend: a gap, an insufficient point with a value (n=3) and a sufficient point.
func sampleTrend() dto.TrendResponse {
	return dto.TrendResponse{
		Period:    dto.PeriodInfo{From: "2026-10-18", To: "2026-10-20", Granularity: "day", Timezone: analytics.TZ},
		MinSample: analytics.MinSample,
		Points:    []string{"2026-10-18", "2026-10-19", "2026-10-20"},
		Series: []dto.TrendSeries{{
			Characteristic: dto.CharacteristicRef{ID: 1, Name: "c1", Core: true},
			Values: []dto.ValuePoint{
				{Value: nil, Employees: 0, Attempts: 0, Insufficient: true},
				{Value: fptr(61.4), Employees: 3, Attempts: 4, Insufficient: true},
				{Value: fptr(70), Employees: 12, Attempts: 30, Insufficient: false},
			},
		}},
	}
}

// analyticsEnv is a server wired to a fake repo plus signed-in users of every role.
type analyticsEnv struct {
	srv                  *Server
	fake                 *fakeRepo
	emp, mgr, adm        *jar
	empID, mgrID, blockd int64
}

func newAnalyticsEnv(t *testing.T, timeout time.Duration) *analyticsEnv {
	t.Helper()
	_, pool, rdb := testDeps(t) // same isolation as every httpapi DB test (truncate + FLUSHDB)
	t.Cleanup(pool.Close)
	fake := &fakeRepo{trend: sampleTrend()}
	srv := New(Deps{
		Pool: pool, Redis: rdb, Users: users.New(pool), Admin: admin.New(pool),
		Sessions: auth.NewManager(rdb, false), FrontOrigin: "http://localhost:5173",
		Analytics: fake, AnalyticsTimeout: timeout,
	})
	emp := testutil.User(t, pool, "an-emp", "secret-123", model.RoleEmployee, true)
	mgr := testutil.User(t, pool, "an-mgr", "secret-123", model.RoleManager, true)
	adm := testutil.User(t, pool, "an-adm", "secret-123", model.RoleAdmin, true)
	blocked := testutil.User(t, pool, "an-blocked", "secret-123", model.RoleEmployee, true)
	fake.employees = map[int64]string{emp.ID: "active", blocked.ID: "blocked"}
	return &analyticsEnv{
		srv: srv, fake: fake,
		emp: signIn(t, srv, emp), mgr: signIn(t, srv, mgr), adm: signIn(t, srv, adm),
		empID: emp.ID, mgrID: mgr.ID, blockd: blocked.ID,
	}
}

func (e *analyticsEnv) get(t *testing.T, j *jar, path string) (int, string) {
	t.Helper()
	rec := do(t, e.srv, j, "GET", path, nil, "")
	return rec.Code, rec.Body.String()
}

var dashboardPaths = []string{"/api/dashboard/trend", "/api/dashboard/team-status"}

func analyticsPaths(empID int64) []string {
	return []string{
		"/api/analytics/trend",
		"/api/analytics/participation",
		"/api/analytics/departments?departments=1",
		"/api/analytics/employees",
		fmt.Sprintf("/api/analytics/employees/%d", empID),
		"/api/analytics/ranking",
		"/api/analytics/changes",
	}
}

// TestAnalyticsAuthz: plan section 6 matrix over all 9 routes.
func TestAnalyticsAuthz(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	type row struct {
		name string
		j    *jar
		dash int
		anal int
	}
	for _, r := range []row{
		{"anonymous", &jar{}, 401, 401},
		{"employee", e.emp, 403, 403},
		{"manager", e.mgr, 200, 403},
		{"admin", e.adm, 200, 200},
	} {
		for _, p := range dashboardPaths {
			if code, body := e.get(t, r.j, p); code != r.dash {
				t.Errorf("%s %s: %d %s, want %d", r.name, p, code, body, r.dash)
			}
		}
		for _, p := range analyticsPaths(e.empID) {
			if code, body := e.get(t, r.j, p); code != r.anal {
				t.Errorf("%s %s: %d %s, want %d", r.name, p, code, body, r.anal)
			}
		}
	}
}

func addDays(d analytics.Date, n int) string {
	return time.Date(d.Year, d.Month, d.Day+n, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// TestAnalyticsValidation: one subtest per row of the plan section 3 validation table.
func TestAnalyticsValidation(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	today := analyticsToday()
	todayS, tomorrow := addDays(today, 0), addDays(today, 1)
	elevenChars, tenWithDups := "", "characteristics=1&"
	for i := 1; i <= 11; i++ {
		elevenChars += fmt.Sprintf("characteristics=%d&", i)
		if i <= 10 {
			tenWithDups += fmt.Sprintf("characteristics=%d&", i)
		}
	}
	nineDepts := ""
	for i := 1; i <= 9; i++ {
		nineDepts += fmt.Sprintf("departments=%d&", i)
	}
	cases := []struct {
		name, path, field, code string
	}{
		{"unknown period", "/api/analytics/trend?period=5y", "period", "invalid"},
		{"dashboard custom", "/api/dashboard/trend?period=custom&from=" + todayS + "&to=" + todayS, "period", "invalid"},
		{"custom without from", "/api/analytics/trend?period=custom&to=" + todayS, "from", "required"},
		{"custom without to", "/api/analytics/trend?period=custom&from=" + todayS, "to", "required"},
		{"from without custom", "/api/analytics/trend?period=30d&from=" + todayS, "from", "unexpected"},
		{"to without custom (default period)", "/api/analytics/trend?to=" + todayS, "from", "unexpected"},
		{"malformed from", "/api/analytics/trend?period=custom&from=2026-13-01&to=" + todayS, "from", "invalid_date"},
		{"malformed to", "/api/analytics/trend?period=custom&from=" + todayS + "&to=20261020", "to", "invalid_date"},
		{"from after to", "/api/analytics/trend?period=custom&from=" + todayS + "&to=" + addDays(today, -1), "from", "after_to"},
		{"to in future", "/api/analytics/trend?period=custom&from=" + todayS + "&to=" + tomorrow, "to", "in_future"},
		{"span too large", "/api/analytics/trend?period=custom&from=" + addDays(today, -3660) + "&to=" + todayS, "to", "span_too_large"},
		{"too many characteristics", "/api/analytics/trend?" + elevenChars, "characteristics", "too_many"},
		{"non-integer characteristic", "/api/analytics/ranking?characteristics=abc", "characteristics", "invalid"},
		{"unknown characteristic", "/api/analytics/changes?characteristics=1&characteristics=9999", "characteristics", "unknown"},
		{"unknown departmentId", "/api/analytics/employees?departmentId=9999", "departmentId", "unknown"},
		{"malformed departmentId", "/api/analytics/participation?departmentId=abc", "departmentId", "unknown"},
		{"unknown department in set", "/api/analytics/departments?departments=9999", "departments", "unknown"},
		{"no departments", "/api/analytics/departments", "departments", "count"},
		{"nine departments", "/api/analytics/departments?" + nineDepts, "departments", "count"},
		{"limit over 200", "/api/analytics/employees?limit=201", "limit", "invalid"},
		{"negative offset", "/api/analytics/ranking?offset=-1", "offset", "invalid"},
		{"threshold zero", "/api/analytics/changes?threshold=0", "threshold", "invalid"},
		{"threshold over 100", "/api/analytics/changes?threshold=101", "threshold", "invalid"},
		{"bad data", "/api/analytics/employees?data=everything", "data", "invalid"},
		{"search too long", "/api/analytics/employees?search=" + strings.Repeat("%D0%AF", 101), "search", "too_long"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, body := e.get(t, e.adm, c.path)
			var got dto.Error
			_ = json.Unmarshal([]byte(body), &got)
			if code != http.StatusUnprocessableEntity || got.Error != "validation" ||
				!slices.Contains(got.Errors, dto.FieldError{Path: c.field, Code: c.code}) {
				t.Fatalf("%s: %d %s, want 422 {%s %s}", c.path, code, body, c.field, c.code)
			}
		})
	}

	valid := []struct{ name, path string }{
		{"from == to", "/api/analytics/trend?period=custom&from=" + todayS + "&to=" + todayS},
		{"span exactly 3660", "/api/analytics/trend?period=custom&from=" + addDays(today, -3659) + "&to=" + todayS},
		{"ten characteristics with duplicates", "/api/analytics/trend?" + tenWithDups},
		{"search of 100 runes", "/api/analytics/employees?search=" + strings.Repeat("%D0%AF", 100)},
		{"departments none", "/api/analytics/departments?departments=none&departments=1"},
		{"departmentId none", "/api/analytics/ranking?departmentId=none&limit=200&offset=0"},
		{"every preset", "/api/dashboard/trend?period=12m"},
		{"unknown and inapplicable params ignored", "/api/analytics/trend?foo=bar&departmentId=zzz&limit=-5"},
		{"participation ignores characteristics", "/api/analytics/participation?characteristics=abc"},
		{"employee view ignores characteristics", fmt.Sprintf("/api/analytics/employees/%d?characteristics=abc&period=all", e.empID)},
	}
	for _, c := range valid {
		t.Run(c.name, func(t *testing.T) {
			if code, body := e.get(t, e.adm, c.path); code != http.StatusOK {
				t.Fatalf("%s: %d %s, want 200", c.path, code, body)
			}
		})
	}
}

// TestAnalyticsEmployeeNotFound: a manager id and a nonexistent id give 404;
// a malformed id gives 404; a blocked employee's detail is served.
func TestAnalyticsEmployeeNotFound(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	for _, p := range []string{
		fmt.Sprintf("/api/analytics/employees/%d", e.mgrID),
		"/api/analytics/employees/987654",
		"/api/analytics/employees/abc",
	} {
		if code, body := e.get(t, e.adm, p); code != http.StatusNotFound || !strings.Contains(body, `"not_found"`) {
			t.Errorf("%s: %d %s, want 404 not_found", p, code, body)
		}
	}
	code, body := e.get(t, e.adm, fmt.Sprintf("/api/analytics/employees/%d", e.blockd))
	if code != http.StatusOK || !strings.Contains(body, `"status":"blocked"`) {
		t.Fatalf("blocked employee: %d %s", code, body)
	}
}

func trendValues(t *testing.T, body string) []dto.ValuePoint {
	t.Helper()
	var tr dto.TrendResponse
	if err := json.Unmarshal([]byte(body), &tr); err != nil || len(tr.Series) != 1 {
		t.Fatalf("trend json: %v %s", err, body)
	}
	return tr.Series[0].Values
}

// TestAnalyticsManagerRedaction: n<5 values are null for managers, present for admins.
func TestAnalyticsManagerRedaction(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	code, body := e.get(t, e.mgr, "/api/dashboard/trend")
	if code != 200 {
		t.Fatalf("manager: %d %s", code, body)
	}
	m := trendValues(t, body)
	if m[0].Value != nil || m[1].Value != nil || !m[1].Insufficient || m[1].Employees != 3 || m[1].Attempts != 4 ||
		m[2].Value == nil || *m[2].Value != 70 {
		t.Fatalf("manager values: %s", body)
	}
	code, body = e.get(t, e.adm, "/api/dashboard/trend")
	if code != 200 {
		t.Fatalf("admin: %d %s", code, body)
	}
	a := trendValues(t, body)
	if a[0].Value != nil || a[1].Value == nil || *a[1].Value != 61.4 || !a[1].Insufficient {
		t.Fatalf("admin values: %s", body)
	}
}

// TestAnalyticsCache: identical requests hit the repo once; manager and admin
// entries are separate; errors are not cached.
func TestAnalyticsCache(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	for i := 0; i < 2; i++ {
		if code, body := e.get(t, e.adm, "/api/dashboard/trend?period=30d"); code != 200 {
			t.Fatalf("admin %d: %d %s", i, code, body)
		}
	}
	if n := e.fake.calls.Load(); n != 1 {
		t.Fatalf("two identical admin requests: %d repo calls, want 1", n)
	}
	_, body := e.get(t, e.mgr, "/api/dashboard/trend?period=30d")
	if n := e.fake.calls.Load(); n != 2 {
		t.Fatalf("manager after admin: %d repo calls, want 2 (role classes must not share)", n)
	}
	if trendValues(t, body)[1].Value != nil {
		t.Fatalf("manager served the admin cache entry: %s", body)
	}
	e.get(t, e.mgr, "/api/dashboard/trend?period=30d")
	if n := e.fake.calls.Load(); n != 2 {
		t.Fatalf("second manager request: %d repo calls, want 2", n)
	}
	e.get(t, e.adm, "/api/dashboard/trend?period=7d") // different params: miss
	if n := e.fake.calls.Load(); n != 3 {
		t.Fatalf("different params: %d repo calls, want 3", n)
	}

	e.fake.err = errors.New("boom")
	before := e.fake.calls.Load()
	for i := 0; i < 2; i++ {
		if code, body := e.get(t, e.adm, "/api/analytics/changes"); code != 500 || !strings.Contains(body, "db error") {
			t.Fatalf("repo error: %d %s, want 500 db error", code, body)
		}
	}
	if n := e.fake.calls.Load() - before; n != 2 {
		t.Fatalf("errors were cached: %d calls, want 2", n)
	}
}

// TestAnalyticsTimeout: an expired query context answers 503 analytics_timeout on every route.
func TestAnalyticsTimeout(t *testing.T) {
	e := newAnalyticsEnv(t, time.Nanosecond)
	e.fake.block = true
	for _, p := range append(slices.Clone(dashboardPaths), analyticsPaths(e.empID)...) {
		code, body := e.get(t, e.adm, p)
		if code != http.StatusServiceUnavailable || strings.TrimSpace(body) != `{"error":"analytics_timeout"}` {
			t.Errorf("%s: %d %s, want 503 analytics_timeout", p, code, body)
		}
	}
	// Postgres statement cancel (57014) maps to the same 503.
	e.fake.block = false
	e.fake.err = fmt.Errorf("query: %w", &pgconn.PgError{Code: "57014"})
	if code, body := e.get(t, e.adm, "/api/analytics/ranking"); code != http.StatusServiceUnavailable ||
		!strings.Contains(body, "analytics_timeout") {
		t.Fatalf("57014: %d %s, want 503", code, body)
	}
}

// TestAnalyticsEmptyShapes: empty repo results keep their JSON shape (null values, empty lists).
func TestAnalyticsEmptyShapes(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	e.fake.trend = dto.TrendResponse{Points: []string{"2026-10-20"}, Series: []dto.TrendSeries{{
		Values: []dto.ValuePoint{{Value: nil, Employees: 0, Attempts: 0, Insufficient: true}},
	}}}
	checks := map[string][]string{
		"/api/dashboard/trend":         {`"value":null`, `"employees":0`},
		"/api/dashboard/team-status":   {`"tests":[]`, `"items":[]`, `"participationRate":null`, `"employees":0`},
		"/api/analytics/participation": {`"departments":[]`, `"tests":[]`, `"rate":null`},
		"/api/analytics/employees":     {`"total":0`, `"items":[]`},
		"/api/analytics/changes":       {`"total":0`, `"items":[]`, `"threshold":15`},
	}
	for p, wants := range checks {
		code, body := e.get(t, e.adm, p)
		if code != 200 {
			t.Errorf("%s: %d %s", p, code, body)
			continue
		}
		for _, w := range wants {
			if !strings.Contains(body, w) {
				t.Errorf("%s: missing %s in %s", p, w, body)
			}
		}
	}
}

// TestAnalyticsTeamStatusRedaction: per-test rates are null for managers when the
// population is below MinSample; admins keep them.
func TestAnalyticsTeamStatusRedaction(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	e.fake.team = &dto.TeamStatus{KPIs: dto.TeamKPIs{Employees: 3},
		Tests: []dto.TestCompletion{{ID: 9, Title: "t", Completed: 2, Employees: 3, Rate: fptr(0.67)}}}
	if code, body := e.get(t, e.mgr, "/api/dashboard/team-status"); code != 200 || !strings.Contains(body, `"rate":null`) {
		t.Fatalf("manager: %d %s, want rate null", code, body)
	}
	if code, body := e.get(t, e.adm, "/api/dashboard/team-status"); code != 200 || !strings.Contains(body, `"rate":0.67`) {
		t.Fatalf("admin: %d %s, want rate 0.67", code, body)
	}
	e.fake.team.KPIs.Employees = 5
	analyticsFlushCache(t, e)
	if code, body := e.get(t, e.mgr, "/api/dashboard/team-status"); code != 200 || !strings.Contains(body, `"rate":0.67`) {
		t.Fatalf("manager n=5: %d %s, want rate kept", code, body)
	}
}

func analyticsFlushCache(t *testing.T, e *analyticsEnv) {
	t.Helper()
	keys, err := e.srv.rdb.Keys(context.Background(), "analytics:v1:*").Result()
	if err != nil {
		t.Fatalf("keys: %v", err)
	}
	if len(keys) > 0 {
		_ = e.srv.rdb.Del(context.Background(), keys...).Err()
	}
}

// getAsync issues a request in a goroutine and reports its status code.
func (e *analyticsEnv) getAsync(t *testing.T, j *jar, path string, out chan<- int) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	for name, val := range j.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: val})
	}
	go func() {
		e.srv.Handler().ServeHTTP(rec, req)
		out <- rec.Code
	}()
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestAnalyticsSingleFlight: identical concurrent cache misses run the repo once.
func TestAnalyticsSingleFlight(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	e.fake.gate = make(chan struct{})
	codes := make(chan int, 5)
	for i := 0; i < 5; i++ {
		e.getAsync(t, e.adm, "/api/analytics/trend?period=30d", codes)
	}
	waitFor(t, "leader in repo", func() bool { return e.fake.calls.Load() == 1 })
	time.Sleep(100 * time.Millisecond) // followers join the flight
	close(e.fake.gate)
	for i := 0; i < 5; i++ {
		if c := <-codes; c != 200 {
			t.Fatalf("request %d: %d, want 200", i, c)
		}
	}
	if n := e.fake.calls.Load(); n != 1 {
		t.Fatalf("5 identical concurrent requests: %d repo calls, want 1", n)
	}
}

// TestAnalyticsConcurrencyCap: at most analyticsMaxConcurrent uncached computations
// run at once; a request that cannot get a slot before its deadline gets 503.
func TestAnalyticsConcurrencyCap(t *testing.T) {
	e := newAnalyticsEnv(t, 0)
	e.fake.gate = make(chan struct{})
	codes := make(chan int, 3)
	for _, p := range []string{"7d", "30d", "90d"} { // distinct keys: no single-flight
		e.getAsync(t, e.adm, "/api/analytics/trend?period="+p, codes)
	}
	waitFor(t, "two computations", func() bool { return e.fake.inflight.Load() == analyticsMaxConcurrent })
	time.Sleep(100 * time.Millisecond)
	if n := e.fake.calls.Load(); n != analyticsMaxConcurrent {
		t.Fatalf("calls while capped: %d, want %d", n, analyticsMaxConcurrent)
	}
	close(e.fake.gate)
	for i := 0; i < 3; i++ {
		if c := <-codes; c != 200 {
			t.Fatalf("request %d: %d, want 200", i, c)
		}
	}
	if n, m := e.fake.calls.Load(), e.fake.maxFlight.Load(); n != 3 || m != analyticsMaxConcurrent {
		t.Fatalf("calls %d (want 3), peak %d (want %d)", n, m, analyticsMaxConcurrent)
	}

	// All slots taken: the wait is bounded by the request deadline -> 503, repo untouched.
	short := newAnalyticsEnv(t, 50*time.Millisecond)
	for i := 0; i < analyticsMaxConcurrent; i++ {
		analyticsSem <- struct{}{}
	}
	defer func() {
		for i := 0; i < analyticsMaxConcurrent; i++ {
			<-analyticsSem
		}
	}()
	if code, body := short.get(t, short.adm, "/api/analytics/changes"); code != http.StatusServiceUnavailable ||
		!strings.Contains(body, "analytics_timeout") {
		t.Fatalf("saturated: %d %s, want 503 analytics_timeout", code, body)
	}
	if n := short.fake.calls.Load(); n != 0 {
		t.Fatalf("saturated: %d repo calls, want 0", n)
	}
}
