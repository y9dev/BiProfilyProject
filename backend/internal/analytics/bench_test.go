package analytics

// Step 2b bench gate: go test ./internal/analytics -run '^$' -bench BenchmarkTrend -benchtime 50x -v
// Needs an explicit TEST_PG_DSN (a throwaway database is created next to it and
// dropped afterwards). Fixture: 2,000 active employees x 30 attempts x 4 scored
// characteristics (240k score rows), 8 departments, ~3 years of history.
// Reports p50/p95 per query via b.ReportMetric; with ANALYTICS_EXPLAIN=1 it also
// logs EXPLAIN (ANALYZE, BUFFERS) of the sweep.

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	benchOnce sync.Once
	benchPool *pgxpool.Pool
	benchErr  string
	benchDrop func()
)

// benchToday is the injected day of the bench (history ends before it).
var benchToday = Date{2026, time.October, 20}

const benchFixtureSQL = `
SELECT setseed(0.42);
INSERT INTO departments (name, active) SELECT 'Dept ' || g, g <> 8 FROM generate_series(1, 8) g;
INSERT INTO users (login, pass_hash, full_name, role) VALUES ('bench_admin', 'x', 'Bench Admin', 'admin');
INSERT INTO users (login, pass_hash, full_name, role, department_id)
  SELECT 'emp' || g, 'x', 'Employee ' || g, 'employee', 1 + g % 8 FROM generate_series(1, 2000) g;
INSERT INTO characteristics (name) SELECT 'Extra ' || g FROM generate_series(1, 6) g;
INSERT INTO assessments (title, created_by)
  SELECT 'Assessment ' || g, (SELECT id FROM users WHERE login = 'bench_admin') FROM generate_series(1, 30) g;
INSERT INTO attempts (user_id, assessment_id, completed_at)
  SELECT u.id, a.id,
         timestamptz '2023-10-20 04:00Z' + (a.rn - 1) * interval '36 days'
           + floor(random() * 30) * interval '1 day' + random() * interval '10 hours'
    FROM users u
   CROSS JOIN (SELECT id, row_number() OVER (ORDER BY id) AS rn FROM assessments) a
   WHERE u.role = 'employee';
INSERT INTO attempt_scores (attempt_id, characteristic_id, score)
  SELECT t.id, c.id, round((random() * 100)::numeric, 2)
    FROM attempts t
    JOIN (SELECT id, row_number() OVER (ORDER BY id) AS rn FROM characteristics) c
      ON (c.rn + t.id) % 10 < 4;
ANALYZE;`

func benchDB(b *testing.B) *pgxpool.Pool {
	b.Helper()
	benchOnce.Do(func() {
		dsn := os.Getenv("TEST_PG_DSN")
		if dsn == "" {
			benchErr = "TEST_PG_DSN not set (the bench needs an explicit DSN)"
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		pool, drop, err := scratchDatabase(ctx, dsn, "bip_analytics_bench")
		if err != nil {
			benchErr = "no postgres: " + err.Error()
			return
		}
		benchDrop = drop
		start := time.Now()
		if _, err := pool.Exec(ctx, benchFixtureSQL); err != nil {
			benchErr = "fixture: " + err.Error()
			return
		}
		var scores int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM attempt_scores`).Scan(&scores)
		b.Logf("bench fixture: %d score rows in %s", scores, time.Since(start).Round(time.Millisecond))
		benchPool = pool
	})
	if benchPool == nil {
		b.Skip(benchErr)
	}
	return benchPool
}

// timed runs fn b.N times and reports p50/p95 in ms.
func timed(b *testing.B, fn func(ctx context.Context) error) {
	b.Helper()
	durs := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
		start := time.Now()
		err := fn(ctx)
		durs = append(durs, time.Since(start))
		cancel()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	pct := func(p float64) float64 {
		idx := int(float64(len(durs))*p+0.999999) - 1
		idx = max(0, min(idx, len(durs)-1))
		return float64(durs[idx].Microseconds()) / 1000
	}
	b.ReportMetric(pct(0.50), "p50_ms")
	b.ReportMetric(pct(0.95), "p95_ms")
	b.ReportMetric(0, "ns/op")
}

func BenchmarkTrend(b *testing.B) {
	pool := benchDB(b)
	r := &repo{pool: pool}
	var seven []int64
	rows, err := pool.Query(context.Background(), `SELECT id FROM characteristics ORDER BY NOT is_default, id LIMIT 7`)
	if err != nil {
		b.Fatal(err)
	}
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		seven = append(seven, id)
	}
	rows.Close()

	if os.Getenv("ANALYTICS_EXPLAIN") == "1" {
		explainSweep(b, pool, seven, benchToday)
	}

	b.Run("analytics_trend_all_month_7c", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.AnalyticsTrend(ctx, benchToday, TrendQuery{Period: PeriodQuery{Preset: PeriodAll}, CharacteristicIDs: seven})
			return err
		})
	})
	b.Run("analytics_trend_90d_day_7c", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.AnalyticsTrend(ctx, benchToday, TrendQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: seven})
			return err
		})
	})
	b.Run("analytics_trend_90d_default", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.AnalyticsTrend(ctx, benchToday, TrendQuery{Period: PeriodQuery{Preset: Period90d}})
			return err
		})
	})
	b.Run("dashboard_trend_all", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.DashboardTrend(ctx, benchToday, PeriodQuery{Preset: PeriodAll})
			return err
		})
	})
	b.Run("dashboard_team_status", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.TeamStatus(ctx, benchToday)
			return err
		})
	})
	b.Run("analytics_participation_90d", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.Participation(ctx, benchToday, ParticipationQuery{Period: PeriodQuery{Preset: Period90d}})
			return err
		})
	})
	b.Run("analytics_ranking_90d_7c", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.Ranking(ctx, benchToday, RankingQuery{Period: PeriodQuery{Preset: Period90d}, CharacteristicIDs: seven})
			return err
		})
	})
	b.Run("analytics_changes_all_7c", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.Changes(ctx, benchToday, ChangesQuery{Period: PeriodQuery{Preset: PeriodAll}, CharacteristicIDs: seven})
			return err
		})
	})
	b.Run("analytics_employees_list", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.Employees(ctx, benchToday, ListQuery{Data: DataAll})
			return err
		})
	})
	b.Run("analytics_employee_view_all", func(b *testing.B) {
		timed(b, func(ctx context.Context) error {
			_, err := r.Employee(ctx, benchToday, 2, PeriodQuery{Preset: PeriodAll})
			return err
		})
	})
	b.Run("analytics_departments_8d_12m", func(b *testing.B) {
		sels := make([]DepartmentSel, 0, 8)
		for i := int64(1); i <= 8; i++ {
			sels = append(sels, DepartmentSel{ID: i})
		}
		timed(b, func(ctx context.Context) error {
			_, err := r.Departments(ctx, benchToday, DepartmentsQuery{Period: PeriodQuery{Preset: Period12m}, Departments: sels})
			return err
		})
	})
}

// explainSweep logs the company sweep plan for the whole history (ANALYZE, BUFFERS).
func explainSweep(b *testing.B, pool *pgxpool.Pool, cids []int64, to Date) {
	ctx := context.Background()
	q := strings.NewReplacer("{{DK}}", "-1::bigint", "{{POP}}", "").Replace(measSQL)
	explain := func(label string) {
		rows, err := pool.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS) "+q, cids, to.String())
		if err != nil {
			b.Fatal(err)
		}
		defer rows.Close()
		var sb strings.Builder
		for rows.Next() {
			var line string
			_ = rows.Scan(&line)
			sb.WriteString(line + "\n")
		}
		b.Logf("EXPLAIN measurement scan (%s):\n%s", label, sb.String())
	}
	explain("existing indexes")
	// Candidate from plan sec. 5, evaluated on the scratch DB only, then dropped.
	if _, err := pool.Exec(ctx, `CREATE INDEX bench_scores_char ON attempt_scores (characteristic_id, attempt_id) INCLUDE (score); ANALYZE attempt_scores`); err != nil {
		b.Fatal(err)
	}
	explain("with attempt_scores (characteristic_id, attempt_id) INCLUDE (score)")
	if _, err := pool.Exec(ctx, `DROP INDEX bench_scores_char; ANALYZE attempt_scores`); err != nil {
		b.Fatal(err)
	}
}
