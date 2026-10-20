// Package dto is the frozen HTTP contract (plan section 7). Every response and
// request body has an explicit struct with json tags here, and each one has a
// golden fixture in contracts/golden/*.json checked by golden_test.go and by
// frontend/src/api/contract.test.mjs. A contract change updates the DTO, the
// golden file and the frontend parser together.
package dto

import "time"

// ---------- shared ----------

// Error is every non-2xx body: {"error": code, "message"?, "errors"?}.
type Error struct {
	Error   string       `json:"error"`
	Message string       `json:"message,omitempty"`
	Errors  []FieldError `json:"errors,omitempty"`
}

// FieldError points at one invalid field, e.g. questions[2].options[1].effects[0].characteristicId.
type FieldError struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

// DepartmentRef is a department as embedded in other payloads.
type DepartmentRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// UserRef is a user as embedded in other payloads.
type UserRef struct {
	ID       int64  `json:"id"`
	FullName string `json:"fullName"`
}

// Avatar is the onboarding triple; ids come from the avatar catalog.
type Avatar struct {
	Universe string `json:"universe"`
	Gender   string `json:"gender"`
	Style    string `json:"style"`
}

// Score is one normalized (0-100) characteristic result.
// NameI18n is set for default characteristics only ({"ru":..,"kk":..}), else null.
type Score struct {
	CharacteristicID int64             `json:"characteristicId"`
	Name             string            `json:"name"`
	NameI18n         map[string]string `json:"nameI18n"`
	Score            float64           `json:"score"`
}

// ---------- auth, registration, avatar ----------

// RegisterOptions: GET /api/register/options?code=. Departments only when valid.
type RegisterOptions struct {
	Valid       bool            `json:"valid"`
	Reason      string          `json:"reason,omitempty"`
	Departments []DepartmentRef `json:"departments,omitempty"`
}

// RegisterRequest: POST /api/auth/register.
type RegisterRequest struct {
	Code         string `json:"code"`
	Login        string `json:"login"`
	Password     string `json:"password"`
	FullName     string `json:"fullName"`
	JobTitle     string `json:"jobTitle"`
	DepartmentID *int64 `json:"departmentId"`
}

// LoginRequest: POST /api/auth/login.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Me: GET /api/auth/me, and the body of login (200) and register (201).
type Me struct {
	ID              int64          `json:"id"`
	Login           string         `json:"login"`
	FullName        string         `json:"fullName"`
	JobTitle        string         `json:"jobTitle"`
	Department      *DepartmentRef `json:"department"`
	Role            string         `json:"role"`
	Avatar          *Avatar        `json:"avatar"`
	ProfileComplete bool           `json:"profileComplete"`
}

// AvatarCatalog: GET /api/avatar/catalog. Ids only; display names come from i18n.
type AvatarCatalog struct {
	Universes []string `json:"universes"`
	Genders   []string `json:"genders"`
	Styles    []string `json:"styles"`
}

// AvatarRequest: PUT /api/me/avatar (response is Me).
type AvatarRequest = Avatar

// ---------- employee ----------

// ProfileCharacteristic is the mean of one characteristic over completed attempts.
type ProfileCharacteristic struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	NameI18n map[string]string `json:"nameI18n"`
	Mean     float64           `json:"mean"`
	Count    int               `json:"count"`
}

type TimelineEntry struct {
	AttemptID       int64     `json:"attemptId"`
	AssessmentTitle string    `json:"assessmentTitle"`
	CompletedAt     time.Time `json:"completedAt"`
	Scores          []Score   `json:"scores"`
}

type ProfileStats struct {
	Completed int `json:"completed"`
	Available int `json:"available"`
}

// Profile: GET /api/me/profile.
type Profile struct {
	Characteristics []ProfileCharacteristic `json:"characteristics"`
	Timeline        []TimelineEntry         `json:"timeline"`
	Stats           ProfileStats            `json:"stats"`
}

// AssessmentListItem: element of GET /api/assessments (visible to the employee).
type AssessmentListItem struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	QuestionCount int        `json:"questionCount"`
	Completed     bool       `json:"completed"`
	AttemptID     *int64     `json:"attemptId,omitempty"`
}

// TakeView: GET /api/assessments/{id}/take. Has NO effect fields by construction.
type TakeView struct {
	ID          int64          `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Questions   []TakeQuestion `json:"questions"`
}

type TakeQuestion struct {
	ID       int64        `json:"id"`
	Position int          `json:"position"`
	Text     string       `json:"text"`
	Type     string       `json:"type"` // single | multiple | rating
	Options  []TakeOption `json:"options"`
}

type TakeOption struct {
	ID       int64  `json:"id"`
	Position int    `json:"position"`
	Text     string `json:"text"`
}

// SubmitRequest: POST /api/assessments/{id}/submit.
type SubmitRequest struct {
	Answers []SubmitAnswer `json:"answers"`
}

type SubmitAnswer struct {
	QuestionID int64   `json:"questionId"`
	OptionIDs  []int64 `json:"optionIds"`
}

// SubmitResult: 201 body of POST /api/assessments/{id}/submit.
type SubmitResult struct {
	AttemptID int64   `json:"attemptId"`
	Scores    []Score `json:"scores"`
}

// AttemptResult: GET /api/me/attempts/{id} (owner only).
type AttemptResult struct {
	AttemptID       int64     `json:"attemptId"`
	AssessmentID    int64     `json:"assessmentId"`
	AssessmentTitle string    `json:"assessmentTitle"`
	CompletedAt     time.Time `json:"completedAt"`
	Scores          []Score   `json:"scores"`
}

// ---------- manager: characteristics, departments ----------

// Characteristic: element of GET /api/characteristics, body of POST/PUT.
type Characteristic struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	NameI18n  map[string]string `json:"nameI18n"`
	IsDefault bool              `json:"isDefault"`
	InUse     bool              `json:"inUse"`
}

// NameRequest: POST/PUT for characteristics and departments.
type NameRequest struct {
	Name string `json:"name"`
}

// Department: element of GET /api/departments, body of POST/PUT/activate.
type Department struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	UserCount int    `json:"userCount"`
}

// DepartmentDeleteResult: 200 body of DELETE /api/departments/{id} when in use (204 otherwise).
type DepartmentDeleteResult struct {
	Deactivated bool `json:"deactivated"`
}

// ---------- manager: invitations ----------

// Invitation: element of GET /api/invitations, body of POST/PATCH/revoke.
type Invitation struct {
	ID                 int64      `json:"id"`
	Code               string     `json:"code"`
	URL                string     `json:"url"`
	ExpiresAt          *time.Time `json:"expiresAt"`
	MaxRegistrations   int        `json:"maxRegistrations"`
	RegistrationsCount int        `json:"registrationsCount"`
	Status             string     `json:"status"` // active | expired | full | revoked
	CreatedAt          time.Time  `json:"createdAt"`
	CreatedBy          UserRef    `json:"createdBy"`
}

// InvitationCreateRequest: POST /api/invitations. ExpiresAt null = never.
type InvitationCreateRequest struct {
	ExpiresAt        *time.Time `json:"expiresAt"`
	MaxRegistrations int        `json:"maxRegistrations"`
}

// Registrant: element of GET /api/invitations/{id}/registrations.
type Registrant struct {
	FullName   string         `json:"fullName"`
	JobTitle   string         `json:"jobTitle"`
	Department *DepartmentRef `json:"department"`
}

// ---------- manager: assessments ----------

// ManageAssessmentListItem: element of GET /api/manage/assessments.
type ManageAssessmentListItem struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Hidden        bool       `json:"hidden"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	Status        string     `json:"status"` // active | hidden | expired
	QuestionCount int        `json:"questionCount"`
	AttemptCount  int        `json:"attemptCount"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// ManageAssessment: GET /api/manage/assessments/{id}, full read-only definition incl. effects.
type ManageAssessment struct {
	ID           int64            `json:"id"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	Hidden       bool             `json:"hidden"`
	ExpiresAt    *time.Time       `json:"expiresAt"`
	Status       string           `json:"status"`
	AttemptCount int              `json:"attemptCount"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
	Questions    []ManageQuestion `json:"questions"`
}

type ManageQuestion struct {
	ID       int64          `json:"id"`
	Position int            `json:"position"`
	Text     string         `json:"text"`
	Type     string         `json:"type"`
	Options  []ManageOption `json:"options"`
}

type ManageOption struct {
	ID       int64    `json:"id"`
	Position int      `json:"position"`
	Text     string   `json:"text"`
	Effects  []Effect `json:"effects"`
}

// Effect: qualitative effect of an option on a characteristic.
type Effect struct {
	CharacteristicID int64  `json:"characteristicId"`
	Level            string `json:"level"`     // low | moderate | high
	Direction        int    `json:"direction"` // -1 | 1
}

// CreateAssessmentRequest: POST /api/manage/assessments. Order of arrays = position.
type CreateAssessmentRequest struct {
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	ExpiresAt   *time.Time              `json:"expiresAt"`
	Hidden      bool                    `json:"hidden"`
	Questions   []CreateQuestionRequest `json:"questions"`
}

type CreateQuestionRequest struct {
	Text    string                `json:"text"`
	Type    string                `json:"type"`
	Options []CreateOptionRequest `json:"options"`
}

type CreateOptionRequest struct {
	Text    string   `json:"text"`
	Effects []Effect `json:"effects"`
}

// Warning is non-blocking (e.g. code "min_unreachable").
type Warning struct {
	Path             string `json:"path"`
	Code             string `json:"code"`
	CharacteristicID int64  `json:"characteristicId"`
}

// CreateAssessmentResult: 201 body of POST /api/manage/assessments.
type CreateAssessmentResult struct {
	ID       int64     `json:"id"`
	Warnings []Warning `json:"warnings"`
}

// AssessmentMetaRequest: PATCH /api/manage/assessments/{id}.
type AssessmentMetaRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

// VisibilityRequest: PUT /api/manage/assessments/{id}/visibility.
type VisibilityRequest struct {
	Hidden bool `json:"hidden"`
}

// ExpiryRequest: PUT /api/manage/assessments/{id}/expiry and the shape used for invitations.
type ExpiryRequest struct {
	ExpiresAt *time.Time `json:"expiresAt"`
}

// ---------- admin ----------

// AdminUser: element of GET /api/admin/users.
type AdminUser struct {
	ID         int64          `json:"id"`
	Login      string         `json:"login"`
	FullName   string         `json:"fullName"`
	JobTitle   string         `json:"jobTitle"`
	Department *DepartmentRef `json:"department"`
	Role       string         `json:"role"`
	Status     string         `json:"status"` // active | blocked
	CreatedAt  time.Time      `json:"createdAt"`
}

// AdminUserList: GET /api/admin/users.
type AdminUserList struct {
	Items []AdminUser `json:"items"`
	Total int         `json:"total"`
}

// RoleRequest: PUT /api/admin/users/{id}/role. Role is manager | employee.
type RoleRequest struct {
	Role string `json:"role"`
}

// PasswordRequest: POST /api/admin/users/{id}/reset-password.
type PasswordRequest struct {
	Password string `json:"password"`
}
