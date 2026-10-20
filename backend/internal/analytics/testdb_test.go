package analytics

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/migrate"
)

// Package DB tests never touch the database TEST_PG_DSN points at: they connect with
// it (localhost default, like httpapi/auth_test.go), create a throwaway database next
// to it, migrate it, and drop it when the package finishes. Unreachable Postgres = skip.

const defaultTestDSN = "postgres://postgres:postgres@localhost:5432/bigroup?sslmode=disable"

var (
	testPool    *pgxpool.Pool
	testSkipMsg string
)

func TestMain(m *testing.M) {
	cleanup := setupTestDB()
	code := m.Run()
	cleanup()
	if benchDrop != nil {
		benchDrop()
	}
	os.Exit(code)
}

// scratchDatabase creates database `name` next to dsn and returns a pool on it plus a drop func.
func scratchDatabase(ctx context.Context, dsn, prefix string) (*pgxpool.Pool, func(), error) {
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		return nil, nil, err
	}
	name := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		return nil, nil, err
	}
	drop := func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close()
	}
	u, err := url.Parse(dsn)
	if err != nil {
		drop()
		return nil, nil, err
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		drop()
		return nil, nil, err
	}
	if _, err := migrate.Up(ctx, pool); err != nil {
		pool.Close()
		drop()
		return nil, nil, err
	}
	return pool, func() { pool.Close(); drop() }, nil
}

func setupTestDB() func() {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, drop, err := scratchDatabase(ctx, dsn, "bip_analytics")
	if err != nil {
		testSkipMsg = fmt.Sprintf("no postgres scratch database next to TEST_PG_DSN (%v)", err)
		return func() {}
	}
	testPool = pool
	return drop
}

// testDB returns the scratch pool with every seeded table emptied (the default
// characteristics from 002_seed.sql are kept).
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip(testSkipMsg)
	}
	if _, err := testPool.Exec(context.Background(), `
		TRUNCATE audit_log, attempt_scores, attempt_answers, attempts, option_effects, options, questions,
		         assessments, users, invitations, departments RESTART IDENTITY CASCADE;
		DELETE FROM characteristics WHERE NOT is_default;`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return testPool
}
