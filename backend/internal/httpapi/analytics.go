package httpapi

// Dashboard (manager+admin) and analytics (admin) routes, backed by analytics.Repo.
// Every handler: computes `today` once (Asia/Almaty), parses and validates the query
// (422 validation), looks up the Redis cache, calls the repo under
// context.WithTimeout(s.analyticsTimeout), nulls n<MinSample values for managers,
// caches the 200 body and writes it. Errors: 404 not_found, 503 analytics_timeout,
// 500 db error. Unknown query params are ignored.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"bigroup-backend/internal/analytics"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

func (s *Server) registerDashboardRoutes() {
	mgr := routeOpts{roles: rolesManager}
	s.protected("GET /api/dashboard/trend", s.dashboardTrend, mgr)
	s.protected("GET /api/dashboard/team-status", s.dashboardTeamStatus, mgr)
}

func (s *Server) registerAnalyticsRoutes() {
	adm := routeOpts{roles: rolesAdmin}
	s.protected("GET /api/analytics/trend", s.analyticsTrend, adm)
	s.protected("GET /api/analytics/participation", s.analyticsParticipation, adm)
	s.protected("GET /api/analytics/departments", s.analyticsDepartments, adm)
	s.protected("GET /api/analytics/employees", s.analyticsEmployees, adm)
	s.protected("GET /api/analytics/employees/{id}", s.analyticsEmployee, adm)
	s.protected("GET /api/analytics/ranking", s.analyticsRanking, adm)
	s.protected("GET /api/analytics/changes", s.analyticsChanges, adm)
}

// Query defaults and caps (plan section 3).
const (
	analyticsDefaultPeriod = analytics.Period90d
	analyticsDefaultLimit  = 50
	analyticsMaxLimit      = 200
	analyticsMaxSearch     = 100 // runes
	// analyticsMaxConcurrent: uncached analytics computations allowed at once (process-wide).
	analyticsMaxConcurrent = 2
)

// analyticsNow is the request clock; tests may replace it to pin `today`.
var analyticsNow = time.Now

// analyticsToday is the request's Asia/Almaty date, computed once per request.
func analyticsToday() analytics.Date { return analytics.Today(analyticsNow()) }

// ---------- handlers ----------

func (s *Server) dashboardTrend(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	q := r.URL.Query()
	var v fieldErrs
	period := v.period(q, today, false)
	if v.fail(w) {
		return
	}
	serveAnalytics(s, w, r, "dashboard-trend", today, period, redactTrend,
		func(ctx context.Context) (dto.TrendResponse, error) {
			return s.analytics.DashboardTrend(ctx, today, period)
		})
}

func (s *Server) dashboardTeamStatus(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	serveAnalytics(s, w, r, "dashboard-team-status", today, struct{}{}, redactTeamStatus,
		func(ctx context.Context) (dto.TeamStatus, error) {
			return s.analytics.TeamStatus(ctx, today)
		})
}

func (s *Server) analyticsTrend(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	q := r.URL.Query()
	var v fieldErrs
	tq := analytics.TrendQuery{Period: v.period(q, today, true), CharacteristicIDs: v.characteristics(q)}
	if v.fail(w) {
		return
	}
	serveAnalytics(s, w, r, "analytics-trend", today, tq, nil,
		func(ctx context.Context) (dto.AnalyticsTrendResponse, error) {
			return s.analytics.AnalyticsTrend(ctx, today, tq)
		})
}

func (s *Server) analyticsParticipation(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	q := r.URL.Query()
	var v fieldErrs
	pq := analytics.ParticipationQuery{Period: v.period(q, today, true), Department: v.departmentID(q)}
	if v.fail(w) {
		return
	}
	serveAnalytics(s, w, r, "analytics-participation", today, pq, nil,
		func(ctx context.Context) (dto.ParticipationResponse, error) {
			return s.analytics.Participation(ctx, today, pq)
		})
}

func (s *Server) analyticsDepartments(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	q := r.URL.Query()
	var v fieldErrs
	dq := analytics.DepartmentsQuery{
		Period: v.period(q, today, true), CharacteristicIDs: v.characteristics(q), Departments: v.departments(q),
	}
	if v.fail(w) {
		return
	}
	serveAnalytics(s, w, r, "analytics-departments", today, dq, nil,
		func(ctx context.Context) (dto.DepartmentsResponse, error) {
			return s.analytics.Departments(ctx, today, dq)
		})
}

func (s *Server) analyticsEmployees(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	q := r.URL.Query()
	var v fieldErrs
	lq := analytics.ListQuery{
		Search: v.search(q), Department: v.departmentID(q), Data: v.data(q),
		Limit: v.limit(q), Offset: v.offset(q),
	}
	if v.fail(w) {
		return
	}
	serveAnalytics(s, w, r, "analytics-employees", today, lq, nil,
		func(ctx context.Context) (dto.AnalyticsEmployeeList, error) {
			return s.analytics.Employees(ctx, today, lq)
		})
}

func (s *Server) analyticsEmployee(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	id, ok := mgrID(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	q := r.URL.Query()
	var v fieldErrs
	period := v.period(q, today, true)
	if v.fail(w) {
		return
	}
	key := struct {
		ID     int64
		Period analytics.PeriodQuery
	}{id, period}
	serveAnalytics(s, w, r, "analytics-employee", today, key, nil,
		func(ctx context.Context) (dto.EmployeeAnalytics, error) {
			return s.analytics.Employee(ctx, today, id, period)
		})
}

func (s *Server) analyticsRanking(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	q := r.URL.Query()
	var v fieldErrs
	rq := analytics.RankingQuery{
		Period: v.period(q, today, true), CharacteristicIDs: v.characteristics(q), Department: v.departmentID(q),
		Limit: v.limit(q), Offset: v.offset(q),
	}
	if v.fail(w) {
		return
	}
	serveAnalytics(s, w, r, "analytics-ranking", today, rq, nil,
		func(ctx context.Context) (dto.RankingResponse, error) {
			return s.analytics.Ranking(ctx, today, rq)
		})
}

func (s *Server) analyticsChanges(w http.ResponseWriter, r *http.Request) {
	today := analyticsToday()
	q := r.URL.Query()
	var v fieldErrs
	cq := analytics.ChangesQuery{
		Period: v.period(q, today, true), CharacteristicIDs: v.characteristics(q), Department: v.departmentID(q),
		Threshold: v.threshold(q), Limit: v.limit(q), Offset: v.offset(q),
	}
	if v.fail(w) {
		return
	}
	serveAnalytics(s, w, r, "analytics-changes", today, cq, nil,
		func(ctx context.Context) (dto.ChangesResponse, error) {
			return s.analytics.Changes(ctx, today, cq)
		})
}

// ---------- shared pipeline: cache, timeout, error mapping, manager redaction ----------

// serveAnalytics runs one analytics query. params must be the full parsed request
// (it is hashed into the cache key); redact, if set, is applied for managers only.
func serveAnalytics[T any](s *Server, w http.ResponseWriter, r *http.Request, route string, today analytics.Date,
	params any, redact func(*T), call func(ctx context.Context) (T, error)) {
	roleClass := model.RoleAdmin
	if sess := auth.UserOf(r.Context()); sess == nil || sess.Role != model.RoleAdmin {
		roleClass = model.RoleManager // only manager and admin reach these routes
	}
	key := analyticsCacheKey(route, roleClass, today, params)
	if body, ok := s.analyticsCacheGet(r.Context(), key); ok {
		writeRawJSON(w, body)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.analyticsTimeout)
	defer cancel()
	c, leader := analyticsFlights.join(key)
	if leader {
		// The shared computation outlives the leader's client (followers may still wait
		// for it) but stays bounded by the analytics timeout.
		lctx, lcancel := context.WithTimeout(context.WithoutCancel(r.Context()), s.analyticsTimeout)
		body, err := computeAnalytics(s, lctx, route, roleClass, key, redact, call)
		lcancel()
		analyticsFlights.finish(key, c, body, err)
	}
	select {
	case <-c.done:
	case <-ctx.Done():
		writeAnalyticsErr(w, route, ctx.Err())
		return
	}
	if c.err != nil {
		writeAnalyticsErr(w, route, c.err)
		return
	}
	writeRawJSON(w, c.body)
}

// computeAnalytics runs one uncached query under the process-wide concurrency cap,
// applies manager redaction, encodes and caches the 200 body.
func computeAnalytics[T any](s *Server, ctx context.Context, route, roleClass, key string,
	redact func(*T), call func(ctx context.Context) (T, error)) ([]byte, error) {
	select {
	case analyticsSem <- struct{}{}:
		defer func() { <-analyticsSem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	v, err := call(ctx)
	if err != nil {
		return nil, err
	}
	if redact != nil && roleClass == model.RoleManager {
		redact(&v)
	}
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	body = append(body, '\n') // same framing as writeJSON's Encoder
	s.analyticsCacheSet(ctx, key, body)
	return body, nil
}

// analyticsSem caps concurrent uncached analytics computations process-wide, so
// heavy queries cannot exhaust the pgx pool; waiting is bounded by the request deadline.
var analyticsSem = make(chan struct{}, analyticsMaxConcurrent)

// analyticsFlights collapses identical concurrent cache misses (same cache key) into one computation.
var analyticsFlights = &flightGroup{m: map[string]*flightCall{}}

type flightCall struct {
	done chan struct{}
	body []byte
	err  error
}

// flightGroup is a minimal single-flight: the first caller for a key leads, later
// callers wait on the leader's done channel.
type flightGroup struct {
	mu sync.Mutex
	m  map[string]*flightCall
}

func (g *flightGroup) join(key string) (c *flightCall, leader bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if c, ok := g.m[key]; ok {
		return c, false
	}
	c = &flightCall{done: make(chan struct{})}
	g.m[key] = c
	return c, true
}

func (g *flightGroup) finish(key string, c *flightCall, body []byte, err error) {
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
	c.body, c.err = body, err
	close(c.done)
}

func writeRawJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func writeAnalyticsErr(w http.ResponseWriter, route string, err error) {
	var verr *analytics.ValidationError
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &verr):
		writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation", Errors: verr.Errors})
	case errors.Is(err, analytics.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found")
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &pg) && pg.Code == "57014"):
		log.Printf("analytics %s: timeout: %v", route, err)
		writeErr(w, http.StatusServiceUnavailable, "analytics_timeout")
	default:
		log.Printf("analytics %s: %v", route, err)
		writeErr(w, http.StatusInternalServerError, "db error")
	}
}

// redactTrend nulls every value below MinSample (manager privacy rule, plan section 3).
// It copies the slices first so it never mutates data the repo may share.
func redactTrend(t *dto.TrendResponse) {
	t.Series = slices.Clone(t.Series)
	for i := range t.Series {
		t.Series[i].Values = slices.Clone(t.Series[i].Values)
		for j := range t.Series[i].Values {
			p := &t.Series[i].Values[j]
			if p.Insufficient || p.Employees < analytics.MinSample {
				p.Value = nil
				p.Insufficient = true
			}
		}
	}
}

// redactTeamStatus nulls per-test completion rates when the population is below
// MinSample (manager privacy rule).
func redactTeamStatus(t *dto.TeamStatus) {
	if t.KPIs.Employees >= analytics.MinSample {
		return
	}
	t.Tests = slices.Clone(t.Tests)
	for i := range t.Tests {
		t.Tests[i].Rate = nil
	}
}

// analyticsCacheKey: analytics:v1:<route>:<roleClass>:<today>:<hash of canonical params>.
func analyticsCacheKey(route, roleClass string, today analytics.Date, params any) string {
	raw, _ := json.Marshal(params) // struct fields marshal in declaration order: canonical
	sum := sha256.Sum256(raw)
	return "analytics:v1:" + route + ":" + roleClass + ":" + today.String() + ":" + hex.EncodeToString(sum[:16])
}

// analyticsCacheGet is fail-open: any Redis error is a miss.
func (s *Server) analyticsCacheGet(ctx context.Context, key string) ([]byte, bool) {
	if s.rdb == nil {
		return nil, false
	}
	b, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil || len(b) == 0 {
		return nil, false
	}
	return b, true
}

func (s *Server) analyticsCacheSet(ctx context.Context, key string, body []byte) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.Set(ctx, key, body, analytics.CacheTTL).Err(); err != nil {
		log.Printf("analytics cache: %v", err)
	}
}

// ---------- query parsing and validation (plan section 3, "Validation") ----------

// fieldErrs collects 422 field errors across all parameters of one request.
type fieldErrs []dto.FieldError

func (v *fieldErrs) add(path, code string) { *v = append(*v, dto.FieldError{Path: path, Code: code}) }

// fail writes the 422 response when any error was collected.
func (v fieldErrs) fail(w http.ResponseWriter) bool {
	if len(v) == 0 {
		return false
	}
	writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation", Errors: v})
	return true
}

func parseDate(s string) (analytics.Date, bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return analytics.Date{}, false
	}
	return analytics.Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}, true
}

func dateTime(d analytics.Date) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// period parses period/from/to. allowCustom=false (dashboard) rejects period=custom.
func (v *fieldErrs) period(q url.Values, today analytics.Date, allowCustom bool) analytics.PeriodQuery {
	p := analytics.PeriodQuery{Preset: q.Get("period")}
	if p.Preset == "" {
		p.Preset = analyticsDefaultPeriod
	}
	switch p.Preset {
	case analytics.Period7d, analytics.Period30d, analytics.Period90d, analytics.Period12m, analytics.PeriodAll:
		if q.Has("from") || q.Has("to") {
			v.add("from", "unexpected")
		}
		return p
	case analytics.PeriodCustom:
		if !allowCustom {
			v.add("period", "invalid")
			return p
		}
	default:
		v.add("period", "invalid")
		return p
	}
	okFrom, okTo := false, false
	if s := q.Get("from"); s == "" {
		v.add("from", "required")
	} else if p.From, okFrom = parseDate(s); !okFrom {
		v.add("from", "invalid_date")
	}
	if s := q.Get("to"); s == "" {
		v.add("to", "required")
	} else if p.To, okTo = parseDate(s); !okTo {
		v.add("to", "invalid_date")
	}
	if !okFrom || !okTo {
		return p
	}
	from, to := dateTime(p.From), dateTime(p.To)
	switch {
	case from.After(to):
		v.add("from", "after_to")
	case to.After(dateTime(today)):
		v.add("to", "in_future")
	case int(to.Sub(from).Hours()/24)+1 > analytics.MaxSpanDays:
		v.add("to", "span_too_large")
	}
	return p
}

// characteristics parses the repeated `characteristics` param: positive integers,
// deduplicated in order, at most MaxCharacteristics. Existence is checked by the repo.
func (v *fieldErrs) characteristics(q url.Values) []int64 {
	var ids []int64
	seen := map[int64]bool{}
	for _, s := range q["characteristics"] {
		id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil || id <= 0 {
			v.add("characteristics", "invalid")
			return nil
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) > analytics.MaxCharacteristics {
		v.add("characteristics", "too_many")
		return nil
	}
	return ids
}

func parseDepartment(s string) (analytics.DepartmentSel, bool) {
	if s == "none" {
		return analytics.DepartmentSel{None: true}, true
	}
	id, err := strconv.ParseInt(s, 10, 64)
	return analytics.DepartmentSel{ID: id}, err == nil && id > 0
}

// departmentID parses the optional single `departmentId` population filter.
func (v *fieldErrs) departmentID(q url.Values) *analytics.DepartmentSel {
	s := strings.TrimSpace(q.Get("departmentId"))
	if s == "" {
		return nil
	}
	d, ok := parseDepartment(s)
	if !ok {
		v.add("departmentId", "unknown")
		return nil
	}
	return &d
}

// departments parses the repeated `departments` set (1..MaxDepartments, deduplicated).
func (v *fieldErrs) departments(q url.Values) []analytics.DepartmentSel {
	var out []analytics.DepartmentSel
	seen := map[analytics.DepartmentSel]bool{}
	for _, s := range q["departments"] {
		d, ok := parseDepartment(strings.TrimSpace(s))
		if !ok {
			v.add("departments", "unknown")
			return nil
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	if len(out) == 0 || len(out) > analytics.MaxDepartments {
		v.add("departments", "count")
		return nil
	}
	return out
}

func (v *fieldErrs) intParam(q url.Values, name string, def, min, max int) int {
	s := strings.TrimSpace(q.Get(name))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < min || n > max {
		v.add(name, "invalid")
		return def
	}
	return n
}

func (v *fieldErrs) limit(q url.Values) int {
	return v.intParam(q, "limit", analyticsDefaultLimit, 1, analyticsMaxLimit)
}

func (v *fieldErrs) offset(q url.Values) int {
	return v.intParam(q, "offset", 0, 0, int(^uint32(0)>>1))
}

func (v *fieldErrs) threshold(q url.Values) int {
	return v.intParam(q, "threshold", analytics.ChangeThreshold, 1, 100)
}

func (v *fieldErrs) search(q url.Values) string {
	s := strings.TrimSpace(q.Get("search"))
	if utf8.RuneCountInString(s) > analyticsMaxSearch {
		v.add("search", "too_long")
		return ""
	}
	return s
}

func (v *fieldErrs) data(q url.Values) string {
	switch s := q.Get("data"); s {
	case "":
		return analytics.DataAll
	case analytics.DataAll, analytics.DataNoData, analytics.DataStale:
		return s
	default:
		v.add("data", "invalid")
		return analytics.DataAll
	}
}
