package invitations

import (
	"time"

	"bigroup-backend/internal/model"
)

// Status derives the invitation status. Precedence is fixed:
// revoked > expired > full > active. expires_at == now counts as expired.
func Status(inv Invitation, now time.Time) string {
	switch {
	case inv.RevokedAt != nil:
		return model.InviteRevoked
	case inv.ExpiresAt != nil && !inv.ExpiresAt.After(now):
		return model.InviteExpired
	case inv.RegistrationsCount >= inv.MaxRegistrations:
		return model.InviteFull
	default:
		return model.InviteActive
	}
}
