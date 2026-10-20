package analytics

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/httpapi/dto"
)

// fixtureToday is the injected request day of every repo test.
var fixtureToday = Date{2026, time.October, 20}

// almatyAt is a wall-clock time in Asia/Almaty.
func almatyAt(date string, hh, mm int) time.Time {
	dd := d(date)
	return time.Date(dd.Year, dd.Month, dd.Day, hh, mm, 0, 0, Location())
}

type fixture struct {
	pool             *pgxpool.Pool
	repo             *repo
	deptA, deptB     int64    // B is inactive
	e                [8]int64 // e[1..7]; e[6], e[7] have no department
	manager, blocked int64
	t1, t2, t3, tDel int64
	c1, x5, x6, x7   int64 // c1 core; x5 covered by 5; x6 by 3; x7 unused
}

func (f *fixture) exec(t *testing.T, q string, a ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), q, a...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func (f *fixture) id(t *testing.T, q string, a ...any) int64 {
	t.Helper()
	var id int64
	if err := f.pool.QueryRow(context.Background(), q, a...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return id
}

func (f *fixture) user(t *testing.T, login, name, role, status string, dept *int64, created time.Time) int64 {
	return f.id(t, `INSERT INTO users (login, pass_hash, full_name, role, status, department_id, created_at)
		VALUES ($1,'x',$2,$3,$4,$5,$6) RETURNING id`, login, name, role, status, dept, created)
}

func (f *fixture) attempt(t *testing.T, user, assessment int64, at time.Time, scores map[int64]float64) {
	t.Helper()
	aid := f.id(t, `INSERT INTO attempts (user_id, assessment_id, completed_at) VALUES ($1,$2,$3) RETURNING id`, user, assessment, at)
	for cid, s := range scores {
		f.exec(t, `INSERT INTO attempt_scores (attempt_id, characteristic_id, score) VALUES ($1,$2,$3)`, aid, cid, s)
	}
}

// newFixture seeds (today = 2026-10-20, 90d window = 2026-07-23..2026-10-20):
//
//	dept A (active): e1, e2, e3; dept B (inactive): e4, e5; no department: e6, e7.
//	c1 measurements: e1 07-01=50 (pre-window baseline), 10-10 10:00=80 and 10-10 15:00=60
//	(same day, latest wins); e2 08-01=70; e3 10-01=40, 10-15=15 (drop -25);
//	e4 06-01=90 (inactive); e5 09-01=60 on a soft-deleted assessment (counted);
//	e6 2026-09-30T19:30Z = 10-01 00:30 Almaty =30 (midnight boundary); e7 none.
//	Manager and blocked user have c1=0 attempts (excluded).
//	x5: e1 (10-10), e2, e3, e4 (pre-window), e5, e6 -> 5 covered in 90d; x6: e1, e2, e3 (n=3).
func newFixture(t *testing.T) *fixture {
	pool := testDB(t)
	f := &fixture{pool: pool, repo: &repo{pool: pool}}
	f.c1 = f.id(t, `SELECT id FROM characteristics WHERE is_default ORDER BY id LIMIT 1`)
	f.x5 = f.id(t, `INSERT INTO characteristics (name) VALUES ('X5 teamwork') RETURNING id`)
	f.x6 = f.id(t, `INSERT INTO characteristics (name) VALUES ('X6 rare') RETURNING id`)
	f.x7 = f.id(t, `INSERT INTO characteristics (name) VALUES ('X7 unused') RETURNING id`)
	f.deptA = f.id(t, `INSERT INTO departments (name, active) VALUES ('Alpha', TRUE) RETURNING id`)
	f.deptB = f.id(t, `INSERT INTO departments (name, active) VALUES ('Beta', FALSE) RETURNING id`)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	depts := []*int64{nil, &f.deptA, &f.deptA, &f.deptA, &f.deptB, &f.deptB, nil, nil}
	for i := 1; i <= 7; i++ {
		f.e[i] = f.user(t, "emp"+itoa(i), "Emp "+itoa(i), "employee", "active", depts[i], base.AddDate(0, 0, i))
	}
	f.manager = f.user(t, "mgr", "Manager", "manager", "active", &f.deptA, base)
	f.blocked = f.user(t, "blk", "Blocked", "employee", "blocked", &f.deptA, base)

	asmt := func(title string, created time.Time, deleted bool) int64 {
		var del *time.Time
		if deleted {
			del = &created
		}
		return f.id(t, `INSERT INTO assessments (title, created_by, created_at, deleted_at) VALUES ($1,$2,$3,$4) RETURNING id`,
			title, f.manager, created, del)
	}
	f.t1 = asmt("T1", base, false)
	f.t2 = asmt("T2", base.AddDate(0, 1, 0), false)
	f.t3 = asmt("T3", base.AddDate(0, 2, 0), false)
	f.tDel = asmt("Tdel", base.AddDate(0, 3, 0), true)

	type sc = map[int64]float64
	f.attempt(t, f.e[1], f.t1, almatyAt("2026-07-01", 10, 0), sc{f.c1: 50})
	f.attempt(t, f.e[1], f.t2, almatyAt("2026-10-10", 10, 0), sc{f.c1: 80, f.x5: 66.67, f.x6: 10})
	f.attempt(t, f.e[1], f.t3, almatyAt("2026-10-10", 15, 0), sc{f.c1: 60})
	f.attempt(t, f.e[2], f.t1, almatyAt("2026-08-01", 9, 0), sc{f.c1: 70, f.x5: 50, f.x6: 20})
	f.attempt(t, f.e[3], f.t1, almatyAt("2026-10-01", 12, 0), sc{f.c1: 40, f.x5: 40, f.x6: 30})
	f.attempt(t, f.e[3], f.t2, almatyAt("2026-10-15", 12, 0), sc{f.c1: 15})
	f.attempt(t, f.e[4], f.t1, almatyAt("2026-06-01", 12, 0), sc{f.c1: 90, f.x5: 90})
	f.attempt(t, f.e[5], f.tDel, almatyAt("2026-09-01", 12, 0), sc{f.c1: 60, f.x5: 60})
	f.attempt(t, f.e[6], f.t1, time.Date(2026, 9, 30, 19, 30, 0, 0, time.UTC), sc{f.c1: 30, f.x5: 30})
	f.attempt(t, f.manager, f.t1, almatyAt("2026-09-01", 12, 0), sc{f.c1: 0, f.x5: 0})
	f.attempt(t, f.blocked, f.t1, almatyAt("2026-09-01", 12, 0), sc{f.c1: 0, f.x5: 0})
	return f
}

func fp(v float64) *float64 { return &v }

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func eqPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return approx(*a, *b)
}

func fieldErr(err error) []dto.FieldError {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Errors
	}
	return nil
}

// ---------- environment assertions ----------

func TestPGTimezone(t *testing.T) {
	pool := testDB(t)
	var ok bool
	if err := pool.QueryRow(context.Background(),
		`SELECT '2026-01-01 00:00Z'::timestamptz AT TIME ZONE 'Asia/Almaty' = '2026-01-01 05:00'::timestamp`).Scan(&ok); err != nil || !ok {
		t.Fatalf("Postgres Asia/Almaty is not UTC+5 (ok=%v err=%v): update tzdata", ok, err)
	}
}

func TestCoreMatchesNameI18n(t *testing.T) {
	pool := testDB(t)
	var bad int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM characteristics WHERE is_default <> (name_i18n IS NOT NULL)`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("%d characteristics where is_default != (name_i18n IS NOT NULL)", bad)
	}
}

// ---------- as-of sweep vs naive ----------

type meas struct {
	user, cid, aid int64
	at             time.Time
	score          float64
}

// naive recomputes each bucket end independently: per population employee the latest
// measurement with day <= D (by completed_at, attempt id), mean and count.
func naive(t *testing.T, pool *pgxpool.Pool, cid int64, ends []Date) []dto.ValuePoint {
	rows, err := pool.Query(context.Background(), `
		SELECT t.user_id, s.characteristic_id, t.id, t.completed_at, s.score::float8
		  FROM attempts t JOIN users u ON u.id = t.user_id JOIN attempt_scores s ON s.attempt_id = t.id
		 WHERE u.role = 'employee' AND u.status = 'active' AND s.characteristic_id = $1`, cid)
	if err != nil {
		t.Fatal(err)
	}
	var ms []meas
	for rows.Next() {
		var m meas
		if err := rows.Scan(&m.user, &m.cid, &m.aid, &m.at, &m.score); err != nil {
			t.Fatal(err)
		}
		ms = append(ms, m)
	}
	rows.Close()
	out := make([]dto.ValuePoint, len(ends))
	for i, end := range ends {
		latest := map[int64]meas{}
		attempts := 0
		for _, m := range ms {
			if DateOf(m.at).After(end) {
				continue
			}
			attempts++
			l, ok := latest[m.user]
			if !ok || m.at.After(l.at) || (m.at.Equal(l.at) && m.aid > l.aid) {
				latest[m.user] = m
			}
		}
		sum := 0.0
		for _, l := range latest {
			sum += l.score
		}
		out[i] = agg{sum: sum, n: len(latest), attempts: attempts}.point(false)
	}
	return out
}

func TestAsOfMatchesNaive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	cases := []struct {
		name string
		q    PeriodQuery
		gran string
	}{
		{"day", PeriodQuery{Preset: Period90d}, GranDay},
		{"day-empty", PeriodQuery{Preset: PeriodCustom, From: d("2026-01-01"), To: d("2026-01-10")}, GranDay},
		{"week", PeriodQuery{Preset: PeriodCustom, From: d("2026-01-01"), To: fixtureToday}, GranWeek},
		{"week-all", PeriodQuery{Preset: PeriodAll}, GranWeek},
		{"month", PeriodQuery{Preset: PeriodCustom, From: d("2025-01-01"), To: fixtureToday}, GranMonth},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := f.repo.AnalyticsTrend(ctx, fixtureToday, TrendQuery{Period: c.q, CharacteristicIDs: []int64{f.c1, f.x5, f.x6, f.x7}})
			if err != nil {
				t.Fatal(err)
			}
			if res.Period.Granularity != c.gran {
				t.Fatalf("granularity %s, want %s", res.Period.Granularity, c.gran)
			}
			ends := make([]Date, len(res.Points))
			for i, p := range res.Points {
				ends[i] = d(p)
			}
			for _, s := range res.Series {
				want := naive(t, f.pool, s.Characteristic.ID, ends)
				for i := range ends {
					g, w := s.Values[i], want[i]
					if !eqPtr(g.Value, w.Value) || g.Employees != w.Employees || g.Attempts != w.Attempts || g.Insufficient != w.Insufficient {
						t.Errorf("char %d at %s: sweep %+v (v=%v), naive %+v (v=%v)", s.Characteristic.ID, res.Points[i], g, deref(g.Value), w, deref(w.Value))
					}
				}
			}
		})
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// ---------- trend, hand-computed ----------

func TestTrendHandComputed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	res, err := f.repo.AnalyticsTrend(ctx, fixtureToday, TrendQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Period.From != "2026-07-23" || res.Period.To != "2026-10-20" || len(res.Points) != 90 {
		t.Fatalf("period %+v points %d", res.Period, len(res.Points))
	}
	at := func(date string) dto.ValuePoint {
		for i, p := range res.Points {
			if p == date {
				return res.Series[0].Values[i]
			}
		}
		t.Fatalf("no point %s", date)
		return dto.ValuePoint{}
	}
	// 07-23: pre-window baselines e1=50, e4=90.
	if p := at("2026-07-23"); !eqPtr(p.Value, fp(70)) || p.Employees != 2 || p.Attempts != 2 || !p.Insufficient {
		t.Errorf("07-23 %+v", p)
	}
	// 09-30: e6's 19:30Z attempt belongs to 10-01 Almaty.
	if p := at("2026-09-30"); !eqPtr(p.Value, fp(67.5)) || p.Employees != 4 {
		t.Errorf("09-30 %+v", p)
	}
	if p := at("2026-10-01"); !eqPtr(p.Value, fp(340.0/6)) || p.Employees != 6 || p.Insufficient {
		t.Errorf("10-01 %+v", p)
	}
	// 10-10: e1 same-day double -> 60 wins; e1 50->60.
	if p := at("2026-10-10"); !eqPtr(p.Value, fp(350.0/6)) || p.Attempts != 8 {
		t.Errorf("10-10 %+v", p)
	}
	if p := at("2026-10-20"); !eqPtr(p.Value, fp(325.0/6)) || p.Employees != 6 || p.Attempts != 9 {
		t.Errorf("10-20 %+v", p)
	}
	// Summary: 7 active employees, 6 measured on c1, 7 attempts in window
	// (e1 x2, e2, e3 x2, e5, e6).
	if res.Summary != (dto.TrendSummary{Employees: 7, MeasuredEmployees: 6, AttemptsInPeriod: 7}) {
		t.Errorf("summary %+v", res.Summary)
	}
	// Default selection: 4 core + x5 (x6 has n=3).
	want := []int64{}
	for _, c := range res.Characteristics {
		if c.Core {
			want = append(want, c.ID)
		}
	}
	want = append(want, f.x5)
	if !reflect.DeepEqual(res.DefaultIDs, want) {
		t.Errorf("defaultIds %v, want %v", res.DefaultIDs, want)
	}
	for _, c := range res.Characteristics {
		switch c.ID {
		case f.x5:
			if c.Employees != 5 || c.Attempts != 5 || !c.Eligible {
				t.Errorf("x5 coverage %+v", c)
			}
		case f.x6:
			if c.Employees != 3 || c.Eligible {
				t.Errorf("x6 coverage %+v", c)
			}
		case f.x7:
			if c.Employees != 0 || c.Eligible {
				t.Errorf("x7 coverage %+v", c)
			}
		}
		if c.Core != (c.NameI18n != nil) {
			t.Errorf("core/nameI18n mismatch %+v", c)
		}
	}
	// Omitted characteristics -> series follow the default selection.
	def, err := f.repo.AnalyticsTrend(ctx, fixtureToday, TrendQuery{Period: PeriodQuery{Preset: Period90d}})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(def.Series); got != 5 {
		t.Errorf("default series %d, want 5", got)
	}
	// Dashboard: core only, custom rejected.
	dash, err := f.repo.DashboardTrend(ctx, fixtureToday, PeriodQuery{Preset: Period30d})
	if err != nil || len(dash.Series) != 4 || len(dash.Points) != 30 {
		t.Errorf("dashboard series=%d points=%d err=%v", len(dash.Series), len(dash.Points), err)
	}
	_, err = f.repo.DashboardTrend(ctx, fixtureToday, PeriodQuery{Preset: PeriodCustom, From: d("2026-10-01"), To: d("2026-10-02")})
	if fe := fieldErr(err); len(fe) != 1 || fe[0] != (dto.FieldError{Path: "period", Code: "invalid"}) {
		t.Errorf("dashboard custom: %v", err)
	}
	// Unknown characteristic -> 422.
	_, err = f.repo.AnalyticsTrend(ctx, fixtureToday, TrendQuery{CharacteristicIDs: []int64{f.c1, 99999}})
	if fe := fieldErr(err); len(fe) != 1 || fe[0] != (dto.FieldError{Path: "characteristics", Code: "unknown"}) {
		t.Errorf("unknown characteristic: %v", err)
	}
	// all: earliest population measurement (e4, 06-01).
	all, err := f.repo.DashboardTrend(ctx, fixtureToday, PeriodQuery{Preset: PeriodAll})
	if err != nil || all.Period.From != "2026-06-01" {
		t.Errorf("all from %s err %v", all.Period.From, err)
	}
}

func TestEmptyDB(t *testing.T) {
	r := &repo{pool: testDB(t)}
	ctx := context.Background()
	tr, err := r.DashboardTrend(ctx, fixtureToday, PeriodQuery{Preset: PeriodAll})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Period.From != fixtureToday.String() || len(tr.Points) != 1 || len(tr.Series) != 4 {
		t.Fatalf("empty trend %+v", tr)
	}
	for _, s := range tr.Series {
		if v := s.Values[0]; v.Value != nil || v.Employees != 0 || !v.Insufficient {
			t.Errorf("empty point %+v", v)
		}
	}
	ts, err := r.TeamStatus(ctx, fixtureToday)
	if err != nil || ts.KPIs.Employees != 0 || ts.KPIs.ParticipationRate != nil || len(ts.Attention.Items) != 0 || len(ts.Tests) != 0 {
		t.Errorf("empty team status %+v err %v", ts, err)
	}
	p, err := r.Participation(ctx, fixtureToday, ParticipationQuery{Period: PeriodQuery{Preset: Period90d}})
	if err != nil || p.Company.Employees != 0 || len(p.Departments) != 0 || len(p.Tests) != 0 {
		t.Errorf("empty participation %+v err %v", p, err)
	}
}

// ---------- team status ----------

func TestTeamStatus(t *testing.T) {
	f := newFixture(t)
	ts, err := f.repo.TeamStatus(context.Background(), fixtureToday)
	if err != nil {
		t.Fatal(err)
	}
	k := ts.KPIs
	if k.Employees != 7 || k.Participated != 6 || k.NoAttempts != 1 || k.ActiveRecently != 3 || k.Inactive != 3 || !eqPtr(k.ParticipationRate, fp(6.0/7)) {
		t.Errorf("kpis %+v", k)
	}
	if ts.AsOf != "2026-10-20" || ts.InactiveDays != 30 || ts.DropThreshold != 15 {
		t.Errorf("header %+v", ts)
	}
	// Visible tests newest first: T3, T2, T1 (Tdel hidden). Completed by population only.
	var got []string
	for _, tc := range ts.Tests {
		got = append(got, tc.Title+":"+itoa(tc.Completed))
	}
	if !reflect.DeepEqual(got, []string{"T3:1", "T2:2", "T1:5"}) {
		t.Errorf("tests %v", got)
	}
	if !eqPtr(ts.Tests[2].Rate, fp(5.0/7)) || ts.Tests[2].Employees != 7 {
		t.Errorf("T1 rate %+v", ts.Tests[2])
	}
	if ts.Attention.Total != 6 || ts.Attention.Counts != (dto.AttentionCounts{NoFirstTest: 1, Inactive: 3, Drop: 2}) {
		t.Errorf("attention counts %+v total %d", ts.Attention.Counts, ts.Attention.Total)
	}
	// Order: drop by magnitude (e3 -25, e1 -20), inactive longest first (e4 141d, e2 80d,
	// e5 49d), then no_first_test (e7).
	var order []int64
	for _, it := range ts.Attention.Items {
		order = append(order, it.User.ID)
	}
	want := []int64{f.e[3], f.e[1], f.e[4], f.e[2], f.e[5], f.e[7]}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("attention order %v, want %v", order, want)
	}
	r0 := ts.Attention.Items[0].Reasons[0]
	if r0.Code != "drop" || *r0.CharacteristicID != f.c1 || !approx(*r0.Delta, -25) || r0.At != "2026-10-15" {
		t.Errorf("e3 drop %+v", r0)
	}
	if r := ts.Attention.Items[1].Reasons[0]; r.Code != "drop" || !approx(*r.Delta, -20) || r.At != "2026-10-10" {
		t.Errorf("e1 same-day drop %+v", r)
	}
	e4 := ts.Attention.Items[2]
	if e4.Reasons[0].Code != "inactive" || *e4.Reasons[0].Days != 141 || e4.Department == nil || e4.Department.Active {
		t.Errorf("e4 %+v", e4)
	}
	e7 := ts.Attention.Items[5]
	if e7.Reasons[0].Code != "no_first_test" || e7.LastAttemptAt != nil || e7.Department != nil {
		t.Errorf("e7 %+v", e7)
	}
}

// ---------- participation ----------

func TestParticipation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, err := f.repo.Participation(ctx, fixtureToday, ParticipationQuery{Period: PeriodQuery{Preset: Period90d}})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Company; c.Employees != 7 || c.Participated != 5 || c.Attempts != 7 || !eqPtr(c.Rate, fp(5.0/7)) {
		t.Errorf("company %+v", c)
	}
	type row struct {
		dept           any
		emp, part, att int
		insufficient   bool
	}
	var rows []row
	for _, d := range p.Departments {
		var name any
		if d.Department != nil {
			name = d.Department.Name
		}
		if d.Rate != nil {
			t.Errorf("rate below MinSample %+v", d)
		}
		rows = append(rows, row{name, d.Employees, d.Participated, d.Attempts, d.Insufficient})
	}
	want := []row{{"Alpha", 3, 3, 5, true}, {"Beta", 2, 1, 1, true}, {nil, 2, 1, 1, true}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("departments %v, want %v", rows, want)
	}
	// Department-bound: company and tests restricted, rate null (n=3).
	pa, err := f.repo.Participation(ctx, fixtureToday, ParticipationQuery{Period: PeriodQuery{Preset: Period90d}, Department: &DepartmentSel{ID: f.deptA}})
	if err != nil {
		t.Fatal(err)
	}
	if c := pa.Company; c.Employees != 3 || c.Participated != 3 || c.Attempts != 5 || c.Rate != nil || len(pa.Departments) != 1 {
		t.Errorf("dept A company %+v rows %d", c, len(pa.Departments))
	}
	for _, tc := range pa.Tests {
		if tc.Title == "T1" && (tc.Completed != 2 || tc.Rate != nil) {
			t.Errorf("dept A T1 %+v", tc)
		}
	}
	// Period-bound custom window including e6's Almaty-midnight attempt on 10-01.
	pc, err := f.repo.Participation(ctx, fixtureToday, ParticipationQuery{Period: PeriodQuery{Preset: PeriodCustom, From: d("2026-10-01"), To: fixtureToday}})
	if err != nil {
		t.Fatal(err)
	}
	if c := pc.Company; c.Participated != 3 || c.Attempts != 5 {
		t.Errorf("custom company %+v", c)
	}
	_, err = f.repo.Participation(ctx, fixtureToday, ParticipationQuery{Department: &DepartmentSel{ID: 99999}})
	if fe := fieldErr(err); len(fe) != 1 || fe[0] != (dto.FieldError{Path: "departmentId", Code: "unknown"}) {
		t.Errorf("unknown department: %v", err)
	}
}

// ---------- departments ----------

func TestDepartmentsView(t *testing.T) {
	f := newFixture(t)
	res, err := f.repo.Departments(context.Background(), fixtureToday, DepartmentsQuery{
		Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1},
		Departments: []DepartmentSel{{ID: f.deptA}, {None: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b := res.Blocks[0]
	last := len(res.Points) - 1
	if v := b.Company.Values[last]; !eqPtr(v.Value, fp(325.0/6)) || v.Employees != 6 {
		t.Errorf("company now %+v", v)
	}
	a := b.Departments[0]
	if a.Department == nil || a.Department.ID != f.deptA || a.Employees != 3 || a.AttemptsInPeriod != 5 {
		t.Errorf("dept A %+v", a)
	}
	if a.Current.Value != nil || !a.Current.Insufficient || a.Current.Employees != 3 || a.Current.VsCompany != nil {
		t.Errorf("dept A current must be withheld %+v", a.Current)
	}
	if n := b.Departments[1]; n.Department != nil || n.Employees != 2 {
		t.Errorf("no-department row %+v", n)
	}
	_, err = f.repo.Departments(context.Background(), fixtureToday, DepartmentsQuery{Departments: []DepartmentSel{{ID: 99999}}})
	if fe := fieldErr(err); len(fe) != 1 || fe[0].Path != "departments" || fe[0].Code != "unknown" {
		t.Errorf("unknown departments: %v", err)
	}
}

// ---------- employees ----------

func TestEmployeesList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ids := func(l dto.AnalyticsEmployeeList) []int64 {
		var out []int64
		for _, e := range l.Items {
			out = append(out, e.ID)
		}
		return out
	}
	all, err := f.repo.Employees(ctx, fixtureToday, ListQuery{Data: DataAll})
	if err != nil || all.Total != 7 {
		t.Fatalf("all total %d err %v", all.Total, err)
	}
	nd, _ := f.repo.Employees(ctx, fixtureToday, ListQuery{Data: DataNoData})
	if !reflect.DeepEqual(ids(nd), []int64{f.e[7]}) || nd.Items[0].DataState != DataNoData || nd.Items[0].LastAttemptAt != nil {
		t.Errorf("no_data %v", ids(nd))
	}
	st, _ := f.repo.Employees(ctx, fixtureToday, ListQuery{Data: DataStale})
	if !reflect.DeepEqual(ids(st), []int64{f.e[2], f.e[4], f.e[5]}) {
		t.Errorf("stale %v", ids(st))
	}
	s, _ := f.repo.Employees(ctx, fixtureToday, ListQuery{Search: "emp 1"})
	if !reflect.DeepEqual(ids(s), []int64{f.e[1]}) || s.Items[0].Attempts != 3 || s.Items[0].DataState != DataOK {
		t.Errorf("search %+v", s.Items)
	}
	nodept, _ := f.repo.Employees(ctx, fixtureToday, ListQuery{Department: &DepartmentSel{None: true}})
	if !reflect.DeepEqual(ids(nodept), []int64{f.e[6], f.e[7]}) {
		t.Errorf("no department %v", ids(nodept))
	}
	pg, _ := f.repo.Employees(ctx, fixtureToday, ListQuery{Limit: 2, Offset: 6})
	if pg.Total != 7 || len(pg.Items) != 1 {
		t.Errorf("paging total %d items %d", pg.Total, len(pg.Items))
	}
}

func TestEmployeeView(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	v, err := f.repo.Employee(ctx, fixtureToday, f.e[1], PeriodQuery{Preset: PeriodCustom, From: d("2026-07-01"), To: d("2026-10-05")})
	if err != nil {
		t.Fatal(err)
	}
	if v.AsOf != "2026-10-05" || len(v.Current) != 1 {
		t.Fatalf("asOf %s current %d", v.AsOf, len(v.Current))
	}
	c := v.Current[0]
	if c.Characteristic.ID != f.c1 || c.Score != 50 || !c.MeasuredAt.Equal(almatyAt("2026-07-01", 10, 0)) {
		t.Errorf("current %+v", c)
	}
	if !eqPtr(c.Company.Value, fp(340.0/6)) || c.Company.Employees != 6 || c.Company.Attempts != 6 {
		t.Errorf("company %+v", c.Company)
	}
	if c.Department.Value != nil || c.Department.Employees != 3 || !c.Department.Insufficient {
		t.Errorf("department %+v", c.Department)
	}
	if len(v.History) != 1 || v.History[0].AssessmentTitle != "T1" || len(v.History[0].Scores) != 1 {
		t.Errorf("history %+v", v.History)
	}
	if v.User.Department == nil || v.User.Department.ID != f.deptA || v.User.Status != "active" || v.User.LastAttemptAt == nil {
		t.Errorf("user %+v", v.User)
	}
	// As of today: every measured characteristic (c1, x5, x6), core first.
	now, err := f.repo.Employee(ctx, fixtureToday, f.e[1], PeriodQuery{Preset: Period90d})
	if err != nil {
		t.Fatal(err)
	}
	var cids []int64
	for _, c := range now.Current {
		cids = append(cids, c.Characteristic.ID)
	}
	if !reflect.DeepEqual(cids, []int64{f.c1, f.x5, f.x6}) || now.Current[0].Score != 60 || len(now.History) != 2 {
		t.Errorf("e1 now %v score %v history %d", cids, now.Current[0].Score, len(now.History))
	}
	// Blocked employee: allowed, status blocked; manager / missing: not found.
	b, err := f.repo.Employee(ctx, fixtureToday, f.blocked, PeriodQuery{Preset: Period90d})
	if err != nil || b.User.Status != "blocked" || len(b.Current) != 2 {
		t.Errorf("blocked %+v err %v", b.User, err)
	}
	for _, id := range []int64{f.manager, 99999} {
		if _, err := f.repo.Employee(ctx, fixtureToday, id, PeriodQuery{Preset: Period90d}); !errors.Is(err, ErrNotFound) {
			t.Errorf("user %d: %v, want ErrNotFound", id, err)
		}
	}
}

// ---------- ranking and changes ----------

func TestRanking(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	res, err := f.repo.Ranking(ctx, fixtureToday, RankingQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1, f.x6}})
	if err != nil {
		t.Fatal(err)
	}
	b := res.Blocks[0]
	type rk struct {
		rank int
		id   int64
		s    float64
	}
	var got []rk
	for _, it := range b.Employees.Items {
		got = append(got, rk{it.Rank, it.User.ID, it.Score})
	}
	// e4 90, e2 70, e1 60 = e5 60 (tie shares rank 3, name order), e6 30, e3 15.
	want := []rk{{1, f.e[4], 90}, {2, f.e[2], 70}, {3, f.e[1], 60}, {3, f.e[5], 60}, {5, f.e[6], 30}, {6, f.e[3], 15}}
	if b.Insufficient || b.Employees.Total != 6 || !reflect.DeepEqual(got, want) {
		t.Errorf("c1 ranking %v total %d", got, b.Employees.Total)
	}
	if len(b.Departments) != 0 {
		t.Errorf("departments below MinSample must be omitted: %+v", b.Departments)
	}
	x6 := res.Blocks[1]
	if !x6.Insufficient || x6.Employees.Total != 3 || len(x6.Employees.Items) != 0 {
		t.Errorf("x6 block %+v", x6)
	}
	// As of 09-30: only 4 measured -> insufficient.
	early, err := f.repo.Ranking(ctx, fixtureToday, RankingQuery{Period: PeriodQuery{Preset: PeriodCustom, From: d("2026-09-01"), To: d("2026-09-30")}, CharacteristicIDs: []int64{f.c1}})
	if err != nil || !early.Blocks[0].Insufficient || early.AsOf != "2026-09-30" {
		t.Errorf("early ranking %+v err %v", early, err)
	}
	pg, _ := f.repo.Ranking(ctx, fixtureToday, RankingQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1}, Limit: 2, Offset: 2})
	if items := pg.Blocks[0].Employees.Items; len(items) != 2 || items[0].Rank != 3 {
		t.Errorf("ranking page %+v", items)
	}
}

func TestRankingDepartments(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// Grow dept A to 5 measured members so it is ranked.
	for i := 0; i < 2; i++ {
		u := f.user(t, "extra"+itoa(i), "Extra "+itoa(i), "employee", "active", &f.deptA, time.Now())
		f.attempt(t, u, f.t1, almatyAt("2026-10-02", 12, 0), map[int64]float64{f.c1: 100})
	}
	res, err := f.repo.Ranking(ctx, fixtureToday, RankingQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1}})
	if err != nil {
		t.Fatal(err)
	}
	deps := res.Blocks[0].Departments
	// A: 60, 70, 15, 100, 100 -> 69; attempts e1 3 + e2 1 + e3 2 + 2 = 8.
	if len(deps) != 1 || deps[0].Rank != 1 || deps[0].Department.ID != f.deptA || !approx(deps[0].Value, 69) || deps[0].Employees != 5 || deps[0].Attempts != 8 {
		t.Errorf("department ranking %+v", deps)
	}
}

func TestChanges(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	res, err := f.repo.Changes(ctx, fixtureToday, ChangesQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1}})
	if err != nil {
		t.Fatal(err)
	}
	type ch struct {
		user  int64
		delta float64
	}
	var got []ch
	for _, c := range res.Items {
		got = append(got, ch{c.User.ID, c.Delta})
	}
	// e1 50->80 (+30), e3 40->15 (-25), e1 80->60 (-20, same day).
	want := []ch{{f.e[1], 30}, {f.e[3], -25}, {f.e[1], -20}}
	if res.Threshold != 15 || res.Total != 3 || !reflect.DeepEqual(got, want) {
		t.Errorf("changes %v total %d", got, res.Total)
	}
	if c := res.Items[0]; !c.From.At.Equal(almatyAt("2026-07-01", 10, 0)) || c.From.Score != 50 || c.Department == nil {
		t.Errorf("first change %+v", c)
	}
	hi, _ := f.repo.Changes(ctx, fixtureToday, ChangesQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1}, Threshold: 25})
	if hi.Total != 2 {
		t.Errorf("threshold 25 total %d", hi.Total)
	}
	b, _ := f.repo.Changes(ctx, fixtureToday, ChangesQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1}, Department: &DepartmentSel{ID: f.deptB}})
	if b.Total != 0 {
		t.Errorf("dept B total %d", b.Total)
	}
	// Window excludes the pre-10-11 pairs.
	late, _ := f.repo.Changes(ctx, fixtureToday, ChangesQuery{Period: PeriodQuery{Preset: PeriodCustom, From: d("2026-10-11"), To: fixtureToday}, CharacteristicIDs: []int64{f.c1}})
	if late.Total != 1 || late.Items[0].User.ID != f.e[3] {
		t.Errorf("late window %+v", late.Items)
	}
	pg, _ := f.repo.Changes(ctx, fixtureToday, ChangesQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1}, Limit: 1, Offset: 1})
	if pg.Total != 3 || len(pg.Items) != 1 || pg.Items[0].User.ID != f.e[3] {
		t.Errorf("changes page %+v", pg)
	}
	past, _ := f.repo.Changes(ctx, fixtureToday, ChangesQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: []int64{f.c1}, Offset: 10})
	if past.Total != 3 || len(past.Items) != 0 || past.Items == nil {
		t.Errorf("changes past end %+v", past)
	}
	if _, err := f.repo.Changes(ctx, fixtureToday, ChangesQuery{Threshold: 101}); fieldErr(err) == nil {
		t.Errorf("threshold 101 accepted")
	}
}
