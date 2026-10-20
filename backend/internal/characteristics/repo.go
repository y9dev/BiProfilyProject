// Package characteristics: manager CRUD. Default characteristics are protected
// (no rename, no delete); custom ones are deletable only when unused.
package characteristics

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound         = errors.New("characteristic not found")
	ErrDuplicate        = errors.New("duplicate characteristic name")
	ErrDefaultProtected = errors.New("default_protected")
)

// InUseError carries the assessments that use the characteristic (409 in_use).
type InUseError struct{ Assessments []AssessmentRef }

func (e *InUseError) Error() string { return "in_use" }

type AssessmentRef struct {
	ID    int64
	Title string
}

type Item struct {
	ID        int64
	Name      string
	NameI18n  map[string]string
	IsDefault bool
	InUse     bool
}

type Repository interface {
	List(ctx context.Context) ([]Item, error)
	Create(ctx context.Context, name string) (Item, error)
	Rename(ctx context.Context, id int64, name string) (Item, error) // ErrNotFound, ErrDefaultProtected, ErrDuplicate
	Delete(ctx context.Context, id int64) error                      // ErrNotFound, ErrDefaultProtected, *InUseError
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

const itemSQL = `SELECT c.id, c.name, c.name_i18n, c.is_default,
	EXISTS (SELECT 1 FROM option_effects e WHERE e.characteristic_id = c.id) FROM characteristics c`

func scanItem(row pgx.Row) (Item, error) {
	var it Item
	var raw []byte
	err := row.Scan(&it.ID, &it.Name, &raw, &it.IsDefault, &it.InUse)
	if errors.Is(err, pgx.ErrNoRows) {
		return it, ErrNotFound
	}
	if err != nil {
		return it, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &it.NameI18n)
	}
	return it, nil
}

func (r *repo) get(ctx context.Context, id int64) (Item, error) {
	return scanItem(r.pool.QueryRow(ctx, itemSQL+" WHERE c.id=$1", id))
}

func (r *repo) List(ctx context.Context) ([]Item, error) {
	rows, err := r.pool.Query(ctx, itemSQL+" ORDER BY c.is_default DESC, lower(c.name), c.id")
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
	err := r.pool.QueryRow(ctx, "INSERT INTO characteristics (name) VALUES ($1) RETURNING id", name).Scan(&id)
	if pgCode(err) == "23505" {
		return Item{}, ErrDuplicate
	}
	if err != nil {
		return Item{}, err
	}
	return r.get(ctx, id)
}

func (r *repo) isDefault(ctx context.Context, id int64) (bool, error) {
	var d bool
	err := r.pool.QueryRow(ctx, "SELECT is_default FROM characteristics WHERE id=$1", id).Scan(&d)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return d, err
}

func (r *repo) Rename(ctx context.Context, id int64, name string) (Item, error) {
	def, err := r.isDefault(ctx, id)
	if err != nil {
		return Item{}, err
	}
	if def {
		return Item{}, ErrDefaultProtected
	}
	_, err = r.pool.Exec(ctx, "UPDATE characteristics SET name=$2 WHERE id=$1 AND NOT is_default", id, name)
	if pgCode(err) == "23505" {
		return Item{}, ErrDuplicate
	}
	if err != nil {
		return Item{}, err
	}
	return r.get(ctx, id)
}

func (r *repo) usedBy(ctx context.Context, id int64) ([]AssessmentRef, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT a.id, a.title FROM option_effects e
		JOIN options o ON o.id = e.option_id JOIN questions q ON q.id = o.question_id
		JOIN assessments a ON a.id = q.assessment_id
		WHERE e.characteristic_id=$1 ORDER BY a.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AssessmentRef{}
	for rows.Next() {
		var a AssessmentRef
		if err := rows.Scan(&a.ID, &a.Title); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *repo) Delete(ctx context.Context, id int64) error {
	def, err := r.isDefault(ctx, id)
	if err != nil {
		return err
	}
	if def {
		return ErrDefaultProtected
	}
	_, err = r.pool.Exec(ctx, "DELETE FROM characteristics WHERE id=$1 AND NOT is_default", id)
	if pgCode(err) == "23503" { // FK RESTRICT from option_effects / attempt_scores
		refs, uerr := r.usedBy(ctx, id)
		if uerr != nil {
			return uerr
		}
		return &InUseError{Assessments: refs}
	}
	return err
}
