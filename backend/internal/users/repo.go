// Package users — учётные записи. Пароли сюда приходят уже хешированными
// (см. internal/auth), репозиторий про криптографию ничего не знает.
package users

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/model"
)

var qb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type Repository interface {
	Count(ctx context.Context) (int, error)
	Create(ctx context.Context, u model.User, passHash string) (model.User, error)
	// GetByLogin returns the user and the password hash (login is case-insensitive).
	GetByLogin(ctx context.Context, login string) (model.User, string, error)
	// CreateTx inserts a user inside the caller's transaction (registration with
	// atomic invitation redemption). ErrLoginTaken on a duplicate login.
	CreateTx(ctx context.Context, tx pgx.Tx, u model.User, passHash string) (int64, error)
	GetByID(ctx context.Context, id int64) (model.User, error)
	// SetAvatar stores the triple and sets avatar_set_at = now().
	SetAvatar(ctx context.Context, id int64, a model.Avatar) error
	// ProfileComplete reads avatar_set_at IS NOT NULL (the avatar gate's DB re-read).
	ProfileComplete(ctx context.Context, id int64) (bool, error)
	SetPassword(ctx context.Context, id int64, passHash string) error
	SetRole(ctx context.Context, id int64, role string) error
	SetStatus(ctx context.Context, id int64, status string) error
}

type repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) Repository { return &repo{pool: pool} }

const userCols = "u.id, u.login, u.full_name, u.job_title, u.department_id, d.name, u.invitation_id, u.pass_hash, " +
	"u.role, u.status, u.avatar_universe, u.avatar_gender, u.avatar_style, u.avatar_set_at, u.created_at"

func selectUsers() sq.SelectBuilder {
	return qb.Select(userCols).From("users u").LeftJoin("departments d ON d.id = u.department_id")
}

func scanUser(row interface{ Scan(...any) error }) (model.User, string, error) {
	var u model.User
	var hash string
	var universe, gender, style *string
	var setAt *time.Time
	err := row.Scan(&u.ID, &u.Login, &u.FullName, &u.JobTitle, &u.DepartmentID, &u.DepartmentName, &u.InvitationID, &hash,
		&u.Role, &u.Status, &universe, &gender, &style, &setAt, &u.CreatedAt)
	if err != nil {
		return u, "", err
	}
	if universe != nil && gender != nil && style != nil {
		u.Avatar = &model.Avatar{Universe: *universe, Gender: *gender, Style: *style}
	}
	u.AvatarSetAt = setAt
	return u, hash, nil
}

func (r *repo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n)
	return n, err
}

func (r *repo) Create(ctx context.Context, u model.User, passHash string) (model.User, error) {
	sql, args, err := qb.Insert("users").
		Columns("login", "full_name", "job_title", "department_id", "invitation_id", "pass_hash", "role").
		Values(u.Login, u.FullName, u.JobTitle, u.DepartmentID, u.InvitationID, passHash, u.Role).
		Suffix("RETURNING id").ToSql()
	if err != nil {
		return u, err
	}
	var id int64
	if err := r.pool.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		return u, err
	}
	return r.GetByID(ctx, id)
}

func (r *repo) CreateTx(ctx context.Context, tx pgx.Tx, u model.User, passHash string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO users (login, full_name, job_title, department_id, invitation_id, pass_hash, role)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		u.Login, u.FullName, u.JobTitle, u.DepartmentID, u.InvitationID, passHash, u.Role).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_login_key" {
		return 0, ErrLoginTaken
	}
	return id, err
}

func (r *repo) SetAvatar(ctx context.Context, id int64, a model.Avatar) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET avatar_universe=$2, avatar_gender=$3, avatar_style=$4, avatar_set_at=now() WHERE id=$1`,
		id, a.Universe, a.Gender, a.Style)
	return err
}

func (r *repo) GetByLogin(ctx context.Context, login string) (model.User, string, error) {
	sql, args, err := selectUsers().Where(sq.Eq{"u.login": login}).ToSql()
	if err != nil {
		return model.User{}, "", err
	}
	return scanUser(r.pool.QueryRow(ctx, sql, args...))
}

func (r *repo) GetByID(ctx context.Context, id int64) (model.User, error) {
	sql, args, err := selectUsers().Where(sq.Eq{"u.id": id}).ToSql()
	if err != nil {
		return model.User{}, err
	}
	u, _, err := scanUser(r.pool.QueryRow(ctx, sql, args...))
	return u, err
}

func (r *repo) ProfileComplete(ctx context.Context, id int64) (bool, error) {
	var done bool
	err := r.pool.QueryRow(ctx, "SELECT avatar_set_at IS NOT NULL FROM users WHERE id = $1", id).Scan(&done)
	return done, err
}

func (r *repo) SetPassword(ctx context.Context, id int64, passHash string) error {
	_, err := r.pool.Exec(ctx, "UPDATE users SET pass_hash = $2, alg_ver = 1 WHERE id = $1", id, passHash)
	return err
}

func (r *repo) SetRole(ctx context.Context, id int64, role string) error {
	if role != model.RoleAdmin && role != model.RoleManager && role != model.RoleEmployee {
		return errBadRole
	}
	_, err := r.pool.Exec(ctx, "UPDATE users SET role = $2 WHERE id = $1", id, role)
	return err
}

func (r *repo) SetStatus(ctx context.Context, id int64, status string) error {
	if status != model.UserActive && status != model.UserBlocked {
		return errBadStatus
	}
	_, err := r.pool.Exec(ctx, "UPDATE users SET status = $2 WHERE id = $1", id, status)
	return err
}
