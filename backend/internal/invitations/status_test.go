package invitations

import (
	"testing"
	"time"

	"bigroup-backend/internal/model"
)

func TestStatusPrecedence(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	cases := []struct {
		name string
		inv  Invitation
		want string
	}{
		{"active", Invitation{MaxRegistrations: 5, RegistrationsCount: 1, ExpiresAt: &future}, model.InviteActive},
		{"never expires", Invitation{MaxRegistrations: 5}, model.InviteActive},
		{"full", Invitation{MaxRegistrations: 5, RegistrationsCount: 5, ExpiresAt: &future}, model.InviteFull},
		{"expired", Invitation{MaxRegistrations: 5, ExpiresAt: &past}, model.InviteExpired},
		{"expired boundary expires_at == now", Invitation{MaxRegistrations: 5, ExpiresAt: &now}, model.InviteExpired},
		{"expired beats full", Invitation{MaxRegistrations: 5, RegistrationsCount: 5, ExpiresAt: &past}, model.InviteExpired},
		{"revoked beats expired", Invitation{MaxRegistrations: 5, ExpiresAt: &past, RevokedAt: &past}, model.InviteRevoked},
		{"revoked beats full", Invitation{MaxRegistrations: 5, RegistrationsCount: 5, RevokedAt: &past}, model.InviteRevoked},
		{"revoked beats all", Invitation{MaxRegistrations: 5, RegistrationsCount: 5, ExpiresAt: &past, RevokedAt: &past}, model.InviteRevoked},
	}
	for _, c := range cases {
		if got := Status(c.inv, now); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
