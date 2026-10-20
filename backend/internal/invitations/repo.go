package invitations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) Repository { return &repo{pool: pool} }

const cols = "id, code, created_by, expires_at, max_registrations, registrations_count, revoked_at, created_at"

func scan(row pgx.Row) (Invitation, error) {
	var v Invitation
	err := row.Scan(&v.ID, &v.Code, &v.CreatedBy, &v.ExpiresAt, &v.MaxRegistrations, &v.RegistrationsCount, &v.RevokedAt, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func (r *repo) Create(ctx context.Context, in CreateInput) (Invitation, error) {
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		return Invitation{}, ErrPastExpiry
	}
	for i := 0; i < 5; i++ {
		code, err := GenerateCode()
		if err != nil {
			return Invitation{}, err
		}
		v, err := scan(r.pool.QueryRow(ctx,
			"INSERT INTO invitations (code, created_by, expires_at, max_registrations) VALUES ($1,$2,$3,$4) RETURNING "+cols,
			code, in.CreatedBy, in.ExpiresAt, in.MaxRegistrations))
		if isUnique(err) {
			continue // code collision: retry
		}
		return v, err
	}
	return Invitation{}, errors.New("invitation code collision")
}

func (r *repo) Get(ctx context.Context, id int64) (Invitation, error) {
	return scan(r.pool.QueryRow(ctx, "SELECT "+cols+" FROM invitations WHERE id=$1", id))
}

func (r *repo) List(ctx context.Context) ([]Invitation, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+cols+" FROM invitations ORDER BY created_at DESC, id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Update applies the patch under a row lock. A revoked invitation cannot be patched.
// Lowering max below registrations_count is refused; a future expiry reactivates an expired one.
func (r *repo) Update(ctx context.Context, id int64, p Patch) (Invitation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Invitation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cur, err := scan(tx.QueryRow(ctx, "SELECT "+cols+" FROM invitations WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return Invitation{}, err
	}
	if cur.RevokedAt != nil {
		return Invitation{}, ErrRevoked
	}
	if p.SetExpiry {
		if p.ExpiresAt != nil && !p.ExpiresAt.After(time.Now()) {
			return Invitation{}, ErrPastExpiry
		}
		cur.ExpiresAt = p.ExpiresAt
	}
	if p.MaxRegistrations != nil {
		if *p.MaxRegistrations < cur.RegistrationsCount {
			return Invitation{}, ErrMaxBelowCount
		}
		cur.MaxRegistrations = *p.MaxRegistrations
	}
	v, err := scan(tx.QueryRow(ctx,
		"UPDATE invitations SET expires_at=$2, max_registrations=$3 WHERE id=$1 RETURNING "+cols,
		id, cur.ExpiresAt, cur.MaxRegistrations))
	if err != nil {
		return Invitation{}, err
	}
	return v, tx.Commit(ctx)
}

func (r *repo) Revoke(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, "UPDATE invitations SET revoked_at = COALESCE(revoked_at, now()) WHERE id=$1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM invitations WHERE id=$1 AND registrations_count = 0", id)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		return ErrHasRegistrations
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var one int
		if e := r.pool.QueryRow(ctx, "SELECT 1 FROM invitations WHERE id=$1", id).Scan(&one); errors.Is(e, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return ErrHasRegistrations
	}
	return nil
}

func (r *repo) Registrants(ctx context.Context, id int64) ([]Registrant, error) {
	return r.RegistrantsPage(ctx, id, 0, 0)
}

func (r *repo) RegistrantsPage(ctx context.Context, id int64, limit, offset int) ([]Registrant, error) {
	var one int
	if err := r.pool.QueryRow(ctx, "SELECT 1 FROM invitations WHERE id=$1", id).Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	q := `SELECT u.full_name, u.job_title, u.department_id, d.name FROM users u
	      LEFT JOIN departments d ON d.id = u.department_id
	      WHERE u.invitation_id=$1 ORDER BY u.created_at, u.id`
	args := []any{id}
	if limit > 0 {
		q += " LIMIT $2 OFFSET $3"
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Registrant{}
	for rows.Next() {
		var g Registrant
		if err := rows.Scan(&g.FullName, &g.JobTitle, &g.DepartmentID, &g.DepartmentName); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
