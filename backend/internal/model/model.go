// Package model holds the domain types shared by repositories and services.
// They mirror the database (backend/internal/migrate/migrations/001_schema.sql).
// HTTP response shapes live in internal/httpapi/dto, never here.
package model

import "time"

const (
	RoleAdmin    = "admin"
	RoleManager  = "manager"
	RoleEmployee = "employee"
)

const (
	UserActive  = "active"
	UserBlocked = "blocked"
)

// Avatar is the (universe, gender, style) triple; ids come from internal/avatar.
type Avatar struct {
	Universe string
	Gender   string
	Style    string
}

type User struct {
	ID           int64
	Login        string
	FullName     string
	JobTitle     string
	DepartmentID *int64
	// DepartmentName is filled by reads that join departments (read convenience, not a column).
	DepartmentName *string
	InvitationID   *int64 // nil only for the bootstrap admin
	Role           string
	Status         string
	Avatar         *Avatar // nil until onboarding is done
	AvatarSetAt    *time.Time
	CreatedAt      time.Time
}

// ProfileComplete reports whether the avatar step is done.
func (u User) ProfileComplete() bool { return u.AvatarSetAt != nil }

type Department struct {
	ID        int64
	Name      string
	Active    bool
	CreatedAt time.Time
}

// Invitation statuses (derived, never stored). Precedence: revoked > expired > full > active.
const (
	InviteActive  = "active"
	InviteExpired = "expired"
	InviteFull    = "full"
	InviteRevoked = "revoked"
)

type Invitation struct {
	ID                 int64
	Code               string
	CreatedBy          int64
	ExpiresAt          *time.Time // nil = never
	MaxRegistrations   int
	RegistrationsCount int
	RevokedAt          *time.Time
	CreatedAt          time.Time
}

type Characteristic struct {
	ID        int64
	Name      string
	NameI18n  map[string]string // defaults only: {"ru":..,"kk":..}
	IsDefault bool
	CreatedAt time.Time
}

const (
	QuestionSingle   = "single"
	QuestionMultiple = "multiple"
	QuestionRating   = "rating"
)

const (
	LevelLow      = "low"
	LevelModerate = "moderate"
	LevelHigh     = "high"
)

type Assessment struct {
	ID          int64
	Title       string
	Description string
	Hidden      bool
	ExpiresAt   *time.Time // nil = never
	CreatedBy   int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time // archived (soft delete) when attempts exist
	Questions   []Question
}

type Question struct {
	ID           int64
	AssessmentID int64
	Position     int
	Text         string
	Type         string // single | multiple | rating
	Options      []Option
}

type Option struct {
	ID         int64
	QuestionID int64
	Position   int
	Text       string
	Effects    []Effect
}

// Effect is a qualitative effect of an option on a characteristic.
type Effect struct {
	CharacteristicID int64
	Level            string // low | moderate | high
	Direction        int    // -1 | 1
}

type Attempt struct {
	ID           int64
	UserID       int64
	AssessmentID int64
	CompletedAt  time.Time
	Answers      []AttemptAnswer
	Scores       []Score
}

type AttemptAnswer struct {
	QuestionID int64
	OptionID   int64
}

// Score is a normalized 0-100 result for one characteristic.
type Score struct {
	CharacteristicID int64
	Score            float64
}

type AuditEntry struct {
	ID         int64     `json:"id"`
	OccurredAt time.Time `json:"occurredAt"`
	ActorID    *int64    `json:"actorId,omitempty"`
	ActorRole  string    `json:"actorRole,omitempty"`
	Action     string    `json:"action"`
	EntityType string    `json:"entityType,omitempty"`
	EntityID   string    `json:"entityId,omitempty"`
	IP         string    `json:"ip,omitempty"`
	Result     string    `json:"result"`
}

type AuditList struct {
	Items []AuditEntry `json:"items"`
	Total int          `json:"total"`
}
