// Package departments: CRUD for departments. A department in use is deactivated,
// never deleted, so users keep their department.
package departments

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/model"
)

var (
	ErrNotFound  = errors.New("department not found")
	ErrDuplicate = errors.New("duplicate department name")
)

// Item is a department with its user count.
type Item struct {
	model.Department
	UserCount int
}

type Repository interface {
	ListActive(ctx context.Context) ([]model.Department, error) // list_active.go (3B)
	List(ctx context.Context, includeInactive bool) ([]Item, error)
	Create(ctx context.Context, name string) (Item, error)
	Rename(ctx context.Context, id int64, name string) (Item, error) // ErrNotFound, ErrDuplicate
	// Delete hard-deletes an unused department (deactivated=false) or deactivates a used one.
	Delete(ctx context.Context, id int64) (deactivated bool, err error)
	Activate(ctx context.Context, id int64) (Item, error) // ErrNotFound, ErrDuplicate
}

type repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) Repository { return &repo{pool: pool} }

func pgCode(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code
	}
	return ""
}

const itemSQL = `SELECT d.id, d.name, d.active, d.created_at,
	(SELECT count(*) FROM users u WHERE u.department_id = d.id)`

func scanItem(row pgx.Row) (Item, error) {
	var it Item
	err := row.Scan(&it.ID, &it.Name, &it.Active, &it.CreatedAt, &it.UserCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return it, ErrNotFound
	}
	return it, err
}

func (r *repo) get(ctx context.Context, id int64) (Item, error) {
	return scanItem(r.pool.QueryRow(ctx, itemSQL+" FROM departments d WHERE d.id=$1", id))
}

func (r *repo) List(ctx context.Context, includeInactive bool) ([]Item, error) {
	q := itemSQL + " FROM departments d"
	if !includeInactive {
		q += " WHERE d.active"
	}
	rows, err := r.pool.Query(ctx, q+" ORDER BY lower(d.name), d.id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *repo) Create(ctx context.Context, name string) (Item, error) {
	var id int64
	err := r.pool.QueryRow(ctx, "INSERT INTO departments (name) VALUES ($1) RETURNING id", name).Scan(&id)
	if pgCode(err) == "23505" {
		return Item{}, ErrDuplicate
	}
	if err != nil {
		return Item{}, err
	}
	return r.get(ctx, id)
}

func (r *repo) Rename(ctx context.Context, id int64, name string) (Item, error) {
	tag, err := r.pool.Exec(ctx, "UPDATE departments SET name=$2 WHERE id=$1", id, name)
	if pgCode(err) == "23505" {
		return Item{}, ErrDuplicate
	}
	if err != nil {
		return Item{}, err
	}
	if tag.RowsAffected() == 0 {
		return Item{}, ErrNotFound
	}
	return r.get(ctx, id)
}

func (r *repo) Delete(ctx context.Context, id int64) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM departments WHERE id=$1)", id).Scan(&exists); err != nil {
		return false, err
	}
	if !exists {
		return false, ErrNotFound
	}
	// Unused: hard delete; the users FK (RESTRICT) is the race backstop.
	tag, err := r.pool.Exec(ctx, `DELETE FROM departments d WHERE d.id=$1
		AND NOT EXISTS (SELECT 1 FROM users u WHERE u.department_id = d.id)`, id)
	if err == nil && tag.RowsAffected() == 1 {
		return false, nil
	}
	if err != nil && pgCode(err) != "23503" {
		return false, err
	}
	if _, err := r.pool.Exec(ctx, "UPDATE departments SET active=FALSE WHERE id=$1", id); err != nil {
		return false, err
	}
	return true, nil
}

func (r *repo) Activate(ctx context.Context, id int64) (Item, error) {
	tag, err := r.pool.Exec(ctx, "UPDATE departments SET active=TRUE WHERE id=$1", id)
	if pgCode(err) == "23505" {
		return Item{}, ErrDuplicate
	}
	if err != nil {
		return Item{}, err
	}
	if tag.RowsAffected() == 0 {
		return Item{}, ErrNotFound
	}
	return r.get(ctx, id)
}
