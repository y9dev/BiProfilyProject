package migrate

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCheckReset(t *testing.T) {
	cases := []struct {
		reset bool
		env   string
		want  error
	}{
		{false, "", nil},
		{false, "prod", nil},
		{true, "dev", nil},
		{true, "", ErrResetRefused},
		{true, "prod", ErrResetRefused},
		{true, "DEV", ErrResetRefused},
		{true, "development", ErrResetRefused},
	}
	for _, c := range cases {
		if got := CheckReset(c.reset, c.env); !errors.Is(got, c.want) {
			t.Errorf("CheckReset(%v, %q) = %v, want %v", c.reset, c.env, got, c.want)
		}
	}
}

// scratchDB creates a throwaway database next to TEST_PG_DSN and returns a pool on it.
// Needs an explicit TEST_PG_DSN (never a default), since it creates and drops databases.
func scratchDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no postgres (%v)", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Skipf("no postgres (%v)", err)
	}
	name := fmt.Sprintf("bip_migrate_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatalf("create db: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connect scratch: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close()
	})
	return pool
}

func TestUpFreshThenNoop(t *testing.T) {
	pool := scratchDB(t)
	ctx := context.Background()
	names, _ := Names()

	res, err := Up(ctx, pool)
	if err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if len(res.Applied) != len(names) || len(res.Skipped) != 0 {
		t.Fatalf("first Up: %+v, want all %v applied", res, names)
	}
	res, err = Up(ctx, pool)
	if err != nil {
		t.Fatalf("second Up: %v", err)
	}
	if len(res.Applied) != 0 || len(res.Skipped) != len(names) {
		t.Fatalf("second Up: %+v, want all skipped", res)
	}

	for _, tbl := range []string{"departments", "invitations", "users", "characteristics", "assessments", "questions",
		"options", "option_effects", "attempts", "attempt_answers", "attempt_scores", "audit_log", "schema_migrations"} {
		var ok bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass('public.'||$1) IS NOT NULL", tbl).Scan(&ok); err != nil || !ok {
			t.Errorf("table %s missing (%v)", tbl, err)
		}
	}

	rows, err := pool.Query(ctx, "SELECT name, name_i18n->>'ru', name_i18n->>'kk' FROM characteristics WHERE is_default ORDER BY id")
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var name, ru, kk string
		if err := rows.Scan(&name, &ru, &kk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if ru != name || kk == "" {
			t.Errorf("default %q: ru=%q kk=%q", name, ru, kk)
		}
		n++
	}
	if n != 4 {
		t.Fatalf("default characteristics = %d, want 4", n)
	}
}

func TestUpRefusesLegacySchema(t *testing.T) {
	pool := scratchDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "CREATE TABLE units (id BIGSERIAL PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("legacy table: %v", err)
	}
	_, err := Up(ctx, pool)
	if err == nil || !strings.Contains(err.Error(), "legacy schema detected") {
		t.Fatalf("Up on legacy schema: %v, want guard error", err)
	}
	var users bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.users') IS NOT NULL").Scan(&users); err != nil || users {
		t.Fatalf("baseline must not be applied on a legacy schema (users exists=%v, err=%v)", users, err)
	}

	// The dev reset clears the legacy schema; the baseline then applies.
	if err := Reset(ctx, pool); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := Up(ctx, pool); err != nil {
		t.Fatalf("Up after reset: %v", err)
	}
}
