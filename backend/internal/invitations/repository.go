// Package invitations owns invitation codes and their redemption.
// The Repository interface is frozen in Step 1.3 so that Step 3B (Redeem,
// IsRedeemable, in redeem.go) and Step 5B (the rest, in repo.go) implement
// disjoint methods without editing the same file.
package invitations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"bigroup-backend/internal/model"
)

// Invitation is the domain row (status is derived, see model.Invite* constants).
type Invitation = model.Invitation

// CreateInput: ExpiresAt nil = never. The code is generated server-side.
type CreateInput struct {
	CreatedBy        int64
	ExpiresAt        *time.Time
	MaxRegistrations int
}

// Patch: a nil field is left unchanged. SetExpiry distinguishes "set to never"
// (SetExpiry=true, ExpiresAt=nil) from "not provided" (SetExpiry=false).
type Patch struct {
	SetExpiry        bool
	ExpiresAt        *time.Time
	MaxRegistrations *int
}

// Registrant is a user registered through an invitation.
type Registrant struct {
	FullName       string
	JobTitle       string
	DepartmentID   *int64
	DepartmentName *string
}

var (
	ErrNotFound         = errors.New("invitation not found")
	ErrInviteInvalid    = errors.New("invite_invalid")    // unknown, expired, full or revoked
	ErrMaxBelowCount    = errors.New("max_below_count")   // max lower than registrations_count
	ErrRevoked          = errors.New("revoked")           // patching a revoked invitation
	ErrPastExpiry       = errors.New("past_expiry")       // expiresAt not in the future
	ErrHasRegistrations = errors.New("has_registrations") // delete of a used invitation
)

type Repository interface {
	Redeem(ctx context.Context, tx pgx.Tx, code string) (invitationID int64, err error) // ErrInviteInvalid
	IsRedeemable(ctx context.Context, code string) (bool, error)
	Create(ctx context.Context, in CreateInput) (Invitation, error)
	List(ctx context.Context) ([]Invitation, error)
	Update(ctx context.Context, id int64, p Patch) (Invitation, error) // ErrMaxBelowCount, ErrRevoked, ErrPastExpiry
	Revoke(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error // ErrHasRegistrations
	Registrants(ctx context.Context, id int64) ([]Registrant, error)
	// Additive (5B): paginated registrants (limit<=0 = all) and single read; ErrNotFound if unknown.
	RegistrantsPage(ctx context.Context, id int64, limit, offset int) ([]Registrant, error)
	Get(ctx context.Context, id int64) (Invitation, error)
}
