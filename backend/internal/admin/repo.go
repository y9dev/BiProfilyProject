// Package admin — репозиторий административных сущностей.
// Только чтение/изменение справочников и аудита. Бизнес-правила проверяются
// в httpapi (права роли admin).
package admin

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/model"
)

var qb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type Repository interface {
	// Пользователи
	UsersSearch(ctx context.Context, query, role, status string, limit, offset int) ([]model.User, int, error)
	// Аудит
	AuditList(ctx context.Context, action, entityType string, limit, offset int) (model.AuditList, error)
	// Admin actions (5B). ErrNotFound, ErrLastAdmin.
	Get(ctx context.Context, id int64) (model.User, error)
	ChangeRole(ctx context.Context, id int64, role string) (model.User, error)
	SetBlocked(ctx context.Context, id int64, blocked bool) (model.User, error)
}

type repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) Repository { return &repo{pool: pool} }

const userCols = "u.id, u.login, u.full_name, u.job_title, u.department_id, d.name, u.role, u.status, u.created_at"

func scanUserRow(row interface{ Scan(...any) error }) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Login, &u.FullName, &u.JobTitle, &u.DepartmentID, &u.DepartmentName, &u.Role, &u.Status, &u.CreatedAt)
	return u, err
}

func (r *repo) UsersSearch(ctx context.Context, query, role, status string, limit, offset int) ([]model.User, int, error) {
	b := qb.Select(userCols).From("users u").LeftJoin("departments d ON d.id = u.department_id")
	if query != "" {
		like := "%" + query + "%"
		b = b.Where(sq.Or{sq.ILike{"u.login": like}, sq.ILike{"u.full_name": like}})
	}
	if role != "" {
		b = b.Where(sq.Eq{"u.role": role})
	}
	if status != "" {
		b = b.Where(sq.Eq{"u.status": status})
	}
	b = b.OrderBy("u.full_name", "u.login")
	if limit > 0 {
		b = b.Limit(uint64(limit))
	}
	if offset > 0 {
		b = b.Offset(uint64(offset))
	}
	sql, args, err := b.ToSql()
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.User{}
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// total
	b2 := qb.Select("count(*)").From("users u")
	if query != "" {
		like := "%" + query + "%"
		b2 = b2.Where(sq.Or{sq.ILike{"u.login": like}, sq.ILike{"u.full_name": like}})
	}
	if role != "" {
		b2 = b2.Where(sq.Eq{"u.role": role})
	}
	if status != "" {
		b2 = b2.Where(sq.Eq{"u.status": status})
	}
	sql2, args2, err := b2.ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := r.pool.QueryRow(ctx, sql2, args2...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *repo) AuditList(ctx context.Context, action, entityType string, limit, offset int) (model.AuditList, error) {
	b := qb.Select("id, occurred_at, actor_id, actor_role, action, entity_type, entity_id, ip, result").
		From("audit_log").OrderBy("occurred_at DESC")
	if action != "" {
		b = b.Where(sq.Eq{"action": action})
	}
	if entityType != "" {
		b = b.Where(sq.Eq{"entity_type": entityType})
	}
	if limit > 0 {
		b = b.Limit(uint64(limit))
	}
	if offset > 0 {
		b = b.Offset(uint64(offset))
	}
	sql, args, err := b.ToSql()
	if err != nil {
		return model.AuditList{}, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return model.AuditList{}, err
	}
	defer rows.Close()
	items := []model.AuditEntry{}
	for rows.Next() {
		var a model.AuditEntry
		var occurredAt time.Time
		var actorID *int64
		if err := rows.Scan(&a.ID, &occurredAt, &actorID, &a.ActorRole, &a.Action, &a.EntityType, &a.EntityID, &a.IP, &a.Result); err != nil {
			return model.AuditList{}, err
		}
		a.OccurredAt = occurredAt
		a.ActorID = actorID
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return model.AuditList{}, err
	}
	// total
	b2 := qb.Select("count(*)").From("audit_log")
	if action != "" {
		b2 = b2.Where(sq.Eq{"action": action})
	}
	if entityType != "" {
		b2 = b2.Where(sq.Eq{"entity_type": entityType})
	}
	sql2, args2, err := b2.ToSql()
	if err != nil {
		return model.AuditList{}, err
	}
	var total int
	if err := r.pool.QueryRow(ctx, sql2, args2...).Scan(&total); err != nil {
		return model.AuditList{}, err
	}
	return model.AuditList{Items: items, Total: total}, nil
}

var (
	ErrNotFound  = errors.New("user not found")
	ErrLastAdmin = errors.New("last_admin")
)

// lockedUpdate runs a guarded UPDATE in a transaction that first locks every active
// admin row (SELECT ... FOR UPDATE). Two concurrent "demote/block the other admin"
// requests therefore serialize: the second re-evaluates the EXISTS guard after the
// first commits and fails with ErrLastAdmin instead of leaving no active admin.
func (r *repo) lockedUpdate(ctx context.Context, id int64, sql string, args ...any) (model.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE role='admin' AND status='active' ORDER BY id FOR UPDATE`); err != nil {
		return model.User{}, err
	}
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return model.User{}, err
	}
	if tag.RowsAffected() == 0 {
		return model.User{}, missingOrLastAdmin(ctx, tx, id)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.User{}, err
	}
	return r.get(ctx, id)
}

// ChangeRole sets manager|employee. Demoting an admin requires another active admin.
func (r *repo) ChangeRole(ctx context.Context, id int64, role string) (model.User, error) {
	return r.lockedUpdate(ctx, id, `UPDATE users SET role=$2 WHERE id=$1 AND
		(role <> 'admin' OR EXISTS (SELECT 1 FROM users o WHERE o.role='admin' AND o.status='active' AND o.id <> $1))`, id, role)
}

// SetBlocked blocks/unblocks. Blocking an admin requires another active admin.
func (r *repo) SetBlocked(ctx context.Context, id int64, blocked bool) (model.User, error) {
	status := model.UserActive
	if blocked {
		status = model.UserBlocked
	}
	return r.lockedUpdate(ctx, id, `UPDATE users SET status=$2 WHERE id=$1 AND
		($2 <> 'blocked' OR role <> 'admin' OR EXISTS (SELECT 1 FROM users o WHERE o.role='admin' AND o.status='active' AND o.id <> $1))`, id, status)
}

func missingOrLastAdmin(ctx context.Context, tx pgx.Tx, id int64) error {
	var one int
	if err := tx.QueryRow(ctx, "SELECT 1 FROM users WHERE id=$1", id).Scan(&one); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return ErrLastAdmin
}

func (r *repo) get(ctx context.Context, id int64) (model.User, error) {
	u, err := scanUserRow(r.pool.QueryRow(ctx, "SELECT "+userCols+" FROM users u LEFT JOIN departments d ON d.id = u.department_id WHERE u.id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (r *repo) Get(ctx context.Context, id int64) (model.User, error) { return r.get(ctx, id) }
