package invitations

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Redeem atomically takes one registration slot inside the caller's transaction.
// The conditional UPDATE serializes concurrent redemptions on the row lock, so
// exactly max_registrations callers succeed; the CHECK constraint is the backstop.
// Any non-redeemable code (unknown, revoked, expired, full) yields ErrInviteInvalid.
func (r *repo) Redeem(ctx context.Context, tx pgx.Tx, code string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		UPDATE invitations SET registrations_count = registrations_count + 1
		 WHERE code = $1 AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > now())
		   AND registrations_count < max_registrations
		RETURNING id`, code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrInviteInvalid
	}
	return id, err
}

// IsRedeemable reports whether the code could be redeemed right now (read-only hint
// for GET /api/register/options; Redeem stays authoritative).
func (r *repo) IsRedeemable(ctx context.Context, code string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM invitations
		 WHERE code = $1 AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > now())
		   AND registrations_count < max_registrations)`, code).Scan(&ok)
	return ok, err
}
