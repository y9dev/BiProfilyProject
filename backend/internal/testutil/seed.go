// Package testutil holds SQL seed helpers for integration tests and for
// developing the frontend against a seeded employee (no UI onboarding needed).
// It writes straight to the database: never import it from production code.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/model"
)

// User inserts a user. withAvatar=true sets a valid avatar triple and avatar_set_at
// (profile complete); false leaves the avatar step pending.
func User(t testing.TB, pool *pgxpool.Pool, login, password, role string, withAvatar bool) model.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	var id int64
	err = pool.QueryRow(context.Background(),
		`INSERT INTO users (login, pass_hash, full_name, role) VALUES ($1,$2,$3,$4) RETURNING id`,
		login, hash, "Test "+login, role).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if withAvatar {
		if _, err := pool.Exec(context.Background(),
			`UPDATE users SET avatar_universe='movies', avatar_gender='female', avatar_style='calm', avatar_set_at=now() WHERE id=$1`, id); err != nil {
			t.Fatalf("seed avatar: %v", err)
		}
	}
	return model.User{ID: id, Login: login, Role: role}
}

// Employee is User(..., RoleEmployee, true): a ready-to-use employee with an avatar.
func Employee(t testing.TB, pool *pgxpool.Pool, login, password string) model.User {
	t.Helper()
	return User(t, pool, login, password, model.RoleEmployee, true)
}

// InviteOpts describes a seeded invitation.
type InviteOpts struct {
	Max       int
	ExpiresAt *time.Time
	Revoked   bool
	Count     int
}

// Invitation inserts an invitation created by createdBy and returns its id.
func Invitation(t testing.TB, pool *pgxpool.Pool, createdBy int64, code string, o InviteOpts) int64 {
	t.Helper()
	if o.Max == 0 {
		o.Max = 5
	}
	var revoked *time.Time
	if o.Revoked {
		now := time.Now()
		revoked = &now
	}
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO invitations (code, created_by, expires_at, max_registrations, registrations_count, revoked_at)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		code, createdBy, o.ExpiresAt, o.Max, o.Count, revoked).Scan(&id)
	if err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	return id
}

// Department inserts an active (or inactive) department and returns its id.
func Department(t testing.TB, pool *pgxpool.Pool, name string, active bool) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO departments (name, active) VALUES ($1,$2) RETURNING id`, name, active).Scan(&id); err != nil {
		t.Fatalf("seed department: %v", err)
	}
	return id
}
