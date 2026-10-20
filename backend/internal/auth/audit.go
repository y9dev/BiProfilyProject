package auth

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/model"
)

var qb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// WriteAudit пишет событие в audit_log. Пароли/токены сюда не попадают никогда.
func WriteAudit(ctx context.Context, pool *pgxpool.Pool, e model.AuditEntry) error {
	sql, args, err := qb.Insert("audit_log").
		Columns("actor_id", "actor_role", "action", "entity_type", "entity_id", "ip", "result").
		Values(e.ActorID, e.ActorRole, e.Action, e.EntityType, e.EntityID, e.IP, e.Result).
		ToSql()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, sql, args...)
	return err
}
