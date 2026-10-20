package assessments

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/httpapi/dto"
)

// ErrNotFound: the assessment does not exist or is archived (404).
var ErrNotFound = errors.New("assessment not found")

// Manager statuses (derived). Precedence: hidden > expired > active.
const (
	StatusActive  = "active"
	StatusHidden  = "hidden"
	StatusExpired = "expired"
)

// ManageStatus derives the manager-list status.
func ManageStatus(hidden bool, expiresAt *time.Time, now time.Time) string {
	switch {
	case hidden:
		return StatusHidden
	case expiresAt != nil && !expiresAt.After(now):
		return StatusExpired
	default:
		return StatusActive
	}
}

// ManageRepo is the manager side: list, metadata, visibility, expiry, delete.
// Question structure is immutable: there is deliberately no method touching
// questions, options or option_effects here.
type ManageRepo struct{ pool *pgxpool.Pool }

func NewManageRepo(pool *pgxpool.Pool) *ManageRepo { return &ManageRepo{pool: pool} }

const manageSelect = `SELECT a.id, a.title, a.description, a.hidden, a.expires_at,
	(SELECT count(*) FROM questions q WHERE q.assessment_id = a.id),
	(SELECT count(*) FROM attempts t WHERE t.assessment_id = a.id),
	a.created_at, a.updated_at FROM assessments a`

func scanManage(row pgx.Row) (dto.ManageAssessmentListItem, error) {
	var it dto.ManageAssessmentListItem
	err := row.Scan(&it.ID, &it.Title, &it.Description, &it.Hidden, &it.ExpiresAt,
		&it.QuestionCount, &it.AttemptCount, &it.CreatedAt, &it.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return it, ErrNotFound
	}
	it.Status = ManageStatus(it.Hidden, it.ExpiresAt, time.Now())
	return it, err
}

// List returns every non-archived assessment, newest first.
func (r *ManageRepo) List(ctx context.Context) ([]dto.ManageAssessmentListItem, error) {
	rows, err := r.pool.Query(ctx, manageSelect+" WHERE a.deleted_at IS NULL ORDER BY a.created_at DESC, a.id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dto.ManageAssessmentListItem{}
	for rows.Next() {
		it, err := scanManage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *ManageRepo) item(ctx context.Context, id int64) (dto.ManageAssessmentListItem, error) {
	return scanManage(r.pool.QueryRow(ctx, manageSelect+" WHERE a.id=$1 AND a.deleted_at IS NULL", id))
}

func (r *ManageRepo) touch(ctx context.Context, id int64, set string, args ...any) (dto.ManageAssessmentListItem, error) {
	tag, err := r.pool.Exec(ctx, "UPDATE assessments SET "+set+", updated_at = now() WHERE id=$1 AND deleted_at IS NULL",
		append([]any{id}, args...)...)
	if err != nil {
		return dto.ManageAssessmentListItem{}, err
	}
	if tag.RowsAffected() == 0 {
		return dto.ManageAssessmentListItem{}, ErrNotFound
	}
	return r.item(ctx, id)
}

// UpdateMeta changes title and/or description (nil = unchanged).
func (r *ManageRepo) UpdateMeta(ctx context.Context, id int64, title, description *string) (dto.ManageAssessmentListItem, error) {
	return r.touch(ctx, id, "title = COALESCE($2, title), description = COALESCE($3, description)", title, description)
}

func (r *ManageRepo) SetHidden(ctx context.Context, id int64, hidden bool) (dto.ManageAssessmentListItem, error) {
	return r.touch(ctx, id, "hidden = $2", hidden)
}

// SetExpiry: nil = never expires.
func (r *ManageRepo) SetExpiry(ctx context.Context, id int64, expiresAt *time.Time) (dto.ManageAssessmentListItem, error) {
	return r.touch(ctx, id, "expires_at = $2", expiresAt)
}

// Delete hard-deletes an assessment without attempts and archives (deleted_at) one that has
// attempts, keeping results in profiles and timelines. The row lock conflicts with the
// submit transaction's FOR SHARE, so a concurrent submit either commits first (archive) or
// sees the assessment gone. archived reports which path was taken.
func (r *ManageRepo) Delete(ctx context.Context, id int64) (archived bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var locked int64
	if err := tx.QueryRow(ctx, "SELECT id FROM assessments WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrNotFound
		}
		return false, err
	}
	var attempts int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM attempts WHERE assessment_id=$1", id).Scan(&attempts); err != nil {
		return false, err
	}
	if attempts == 0 {
		if _, err := tx.Exec(ctx, "DELETE FROM assessments WHERE id=$1", id); err != nil {
			var pg *pgconn.PgError
			if !errors.As(err, &pg) || pg.Code != "23503" {
				return false, err
			}
			return false, errors.New("assessment has dependent rows") // FK backstop; cannot happen under the row lock
		}
		return false, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, "UPDATE assessments SET deleted_at = now(), updated_at = now() WHERE id=$1", id); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
