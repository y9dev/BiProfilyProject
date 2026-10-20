// Package migrate applies the SQL migrations embedded in the binary.
// Each file runs once, inside a transaction, and is recorded in schema_migrations,
// so a second boot is a no-op. 000_guard.sql refuses to run on a legacy schema.
package migrate

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var files embed.FS

// advisoryLockKey serializes concurrent boots (several backend replicas).
const advisoryLockKey = 0x6269_7072_6f66 // "biprof"

// Result lists which migration files were applied and which were already present.
type Result struct {
	Applied []string
	Skipped []string
}

// Names returns the embedded migration file names in apply order.
func Names() ([]string, error) {
	entries, err := files.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Up applies every migration not yet recorded in schema_migrations.
func Up(ctx context.Context, pool *pgxpool.Pool) (Result, error) {
	var res Result
	names, err := Names()
	if err != nil {
		return res, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return res, err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", int64(advisoryLockKey)); err != nil {
		return res, err
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", int64(advisoryLockKey))
	}()

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return res, err
	}
	applied := map[string]bool{}
	rows, err := conn.Query(ctx, "SELECT name FROM schema_migrations")
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return res, err
		}
		applied[n] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	for _, n := range names {
		if applied[n] {
			res.Skipped = append(res.Skipped, n)
			continue
		}
		raw, err := files.ReadFile("migrations/" + n)
		if err != nil {
			return res, err
		}
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(raw)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "INSERT INTO schema_migrations (name) VALUES ($1)", n)
			return err
		})
		if err != nil {
			return res, fmt.Errorf("%s: %w", n, err)
		}
		res.Applied = append(res.Applied, n)
	}
	log.Printf("migrate: applied=%v skipped=%v", res.Applied, res.Skipped)
	return res, nil
}

// ErrResetRefused is returned when MIGRATE_RESET is requested outside APP_ENV=dev.
var ErrResetRefused = errors.New("MIGRATE_RESET=1 refused: allowed only with APP_ENV=dev")

// CheckReset decides whether a dev reset may run. It must be called before
// connecting to the database, so a refusal never touches any data.
// reset=false: nothing to do. reset=true requires appEnv to be exactly "dev".
func CheckReset(reset bool, appEnv string) error {
	if !reset {
		return nil
	}
	if appEnv != "dev" {
		return ErrResetRefused
	}
	return nil
}

// Reset drops and recreates the public schema (dev only; gate with CheckReset first).
func Reset(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	return err
}
