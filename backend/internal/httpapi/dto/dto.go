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

// ---------- dashboard / analytics ----------
//
// GET /api/dashboard/{trend,team-status} (manager+admin) and GET /api/analytics/*
// (admin). Dates without a time ("2026-10-20") are Asia/Almaty calendar days and
// are plain strings; instants (completedAt, lastAttemptAt, measuredAt) are RFC 3339.
// Every aggregate carries both employees and attempts. Groups below MinSample (5)
// carry insufficient=true and, for managers and every group view, value=null.

// AnalyticsDepartmentRef is a department in analytics payloads; Active=false marks
// an inactive department. A user's department is *AnalyticsDepartmentRef (null = no department).
type AnalyticsDepartmentRef struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// CharacteristicRef identifies a characteristic. Core comes from characteristics.is_default;
// NameI18n is set exactly for core characteristics.
type CharacteristicRef struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	NameI18n map[string]string `json:"nameI18n"`
	Core     bool              `json:"core"`
}

// PeriodInfo is the resolved window (inclusive dates). Granularity: day | week | month.
type PeriodInfo struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Granularity string `json:"granularity"`
	Timezone    string `json:"timezone"`
}

// ValuePoint is one aggregate: mean score (null = gap, or withheld below MinSample),
// distinct employees, attempts and the small-sample flag.
type ValuePoint struct {
	Value        *float64 `json:"value"`
	Employees    int      `json:"employees"`
	Attempts     int      `json:"attempts"`
	Insufficient bool     `json:"insufficient"`
}

// TrendSeries: Values[i] aligns with TrendResponse.Points[i].
type TrendSeries struct {
	Characteristic CharacteristicRef `json:"characteristic"`
	Values         []ValuePoint      `json:"values"`
}

type TrendSummary struct {
	Employees         int `json:"employees"`
	MeasuredEmployees int `json:"measuredEmployees"`
	AttemptsInPeriod  int `json:"attemptsInPeriod"`
}

// TrendResponse: GET /api/dashboard/trend. Points are bucket end dates.
type TrendResponse struct {
	Period    PeriodInfo    `json:"period"`
	MinSample int           `json:"minSample"`
	Points    []string      `json:"points"`
	Series    []TrendSeries `json:"series"`
	Summary   TrendSummary  `json:"summary"`
}

// CharacteristicCoverage is one entry of the analytics characteristic catalog
// (coverage in the period; Eligible = employees >= MinSample).
type CharacteristicCoverage struct {
	CharacteristicRef
	Employees int  `json:"employees"`
	Attempts  int  `json:"attempts"`
	Eligible  bool `json:"eligible"`
}

// AnalyticsTrendResponse: GET /api/analytics/trend. TrendResponse plus the default
// selection and the characteristic catalog that drives the picker.
type AnalyticsTrendResponse struct {
	TrendResponse
	DefaultIDs      []int64                  `json:"defaultIds"`
	Characteristics []CharacteristicCoverage `json:"characteristics"`
}

// TestCompletion: per-assessment completion. Rate is null when Employees is 0
// (or, in participation, below MinSample).
type TestCompletion struct {
	ID        int64    `json:"id"`
	Title     string   `json:"title"`
	Completed int      `json:"completed"`
	Employees int      `json:"employees"`
	Rate      *float64 `json:"rate"`
}

type TeamKPIs struct {
	Employees         int      `json:"employees"`
	Participated      int      `json:"participated"`
	NoAttempts        int      `json:"noAttempts"`
	ActiveRecently    int      `json:"activeRecently"`
	Inactive          int      `json:"inactive"`
	ParticipationRate *float64 `json:"participationRate"`
}

// Reason explains an attention item. Code: drop | inactive | no_first_test.
// drop sets CharacteristicID, Delta, At (date); inactive sets Days; no_first_test sets nothing.
type Reason struct {
	Code             string   `json:"code"`
	CharacteristicID *int64   `json:"characteristicId,omitempty"`
	Delta            *float64 `json:"delta,omitempty"`
	At               string   `json:"at,omitempty"`
	Days             *int     `json:"days,omitempty"`
}

type AttentionItem struct {
	User          UserRef                 `json:"user"`
	Department    *AnalyticsDepartmentRef `json:"department"`
	LastAttemptAt *time.Time              `json:"lastAttemptAt"`
	Reasons       []Reason                `json:"reasons"`
}

type AttentionCounts struct {
	NoFirstTest int `json:"noFirstTest"`
	Inactive    int `json:"inactive"`
	Drop        int `json:"drop"`
}

// Attention: Items is capped (50); Total and Counts cover everything.
type Attention struct {
	Total  int             `json:"total"`
	Counts AttentionCounts `json:"counts"`
	Items  []AttentionItem `json:"items"`
}

// TeamStatus: GET /api/dashboard/team-status, as of AsOf (today, Asia/Almaty).
type TeamStatus struct {
	AsOf          string           `json:"asOf"`
	InactiveDays  int              `json:"inactiveDays"`
	DropThreshold int              `json:"dropThreshold"`
	KPIs          TeamKPIs         `json:"kpis"`
	Tests         []TestCompletion `json:"tests"`
	Attention     Attention        `json:"attention"`
}

type ParticipationCompany struct {
	Employees    int      `json:"employees"`
	Participated int      `json:"participated"`
	Attempts     int      `json:"attempts"`
	Rate         *float64 `json:"rate"`
}

// ParticipationDepartment: Department null = "No department". Below MinSample: Rate null, Insufficient true.
type ParticipationDepartment struct {
	Department   *AnalyticsDepartmentRef `json:"department"`
	Employees    int                     `json:"employees"`
	Participated int                     `json:"participated"`
	Attempts     int                     `json:"attempts"`
	Rate         *float64                `json:"rate"`
	Insufficient bool                    `json:"insufficient"`
}

// ParticipationResponse: GET /api/analytics/participation.
type ParticipationResponse struct {
	Period      PeriodInfo                `json:"period"`
	Company     ParticipationCompany      `json:"company"`
	Departments []ParticipationDepartment `json:"departments"`
	Tests       []TestCompletion          `json:"tests"`
}

// DepartmentCurrent is the as-of-`to` standing; VsCompany is null if either side is insufficient.
type DepartmentCurrent struct {
	ValuePoint
	VsCompany *float64 `json:"vsCompany"`
}

type DepartmentSeries struct {
	Department       *AnalyticsDepartmentRef `json:"department"`
	Employees        int                     `json:"employees"`
	AttemptsInPeriod int                     `json:"attemptsInPeriod"`
	Values           []ValuePoint            `json:"values"`
	Current          DepartmentCurrent       `json:"current"`
}

type CompanySeries struct {
	Values []ValuePoint `json:"values"`
}

// DepartmentsBlock: one block per requested characteristic.
type DepartmentsBlock struct {
	Characteristic CharacteristicRef  `json:"characteristic"`
	Company        CompanySeries      `json:"company"`
	Departments    []DepartmentSeries `json:"departments"`
}

// DepartmentsResponse: GET /api/analytics/departments.
type DepartmentsResponse struct {
	Period PeriodInfo         `json:"period"`
	Points []string           `json:"points"`
	Blocks []DepartmentsBlock `json:"blocks"`
}

// AnalyticsEmployee: element of GET /api/analytics/employees (active employees only).
// DataState: ok | no_data | stale.
type AnalyticsEmployee struct {
	ID            int64                   `json:"id"`
	FullName      string                  `json:"fullName"`
	JobTitle      string                  `json:"jobTitle"`
	Department    *AnalyticsDepartmentRef `json:"department"`
	Attempts      int                     `json:"attempts"`
	LastAttemptAt *time.Time              `json:"lastAttemptAt"`
	DataState     string                  `json:"dataState"`
}

// AnalyticsEmployeeList: GET /api/analytics/employees.
type AnalyticsEmployeeList struct {
	Total int                 `json:"total"`
	Items []AnalyticsEmployee `json:"items"`
}

// AnalyticsUser: the subject of the employee view. Status: active | blocked.
type AnalyticsUser struct {
	ID            int64                   `json:"id"`
	FullName      string                  `json:"fullName"`
	JobTitle      string                  `json:"jobTitle"`
	Department    *AnalyticsDepartmentRef `json:"department"`
	Status        string                  `json:"status"`
	LastAttemptAt *time.Time              `json:"lastAttemptAt"`
}

// EmployeeCurrent: the employee's latest score as of `to` with company and
// department comparisons (comparison values null below MinSample).
type EmployeeCurrent struct {
	Characteristic CharacteristicRef `json:"characteristic"`
	Score          float64           `json:"score"`
	MeasuredAt     time.Time         `json:"measuredAt"`
	Company        ValuePoint        `json:"company"`
	Department     ValuePoint        `json:"department"`
}

// EmployeeAnalytics: GET /api/analytics/employees/{id}. AsOf = range end `to`;
// Current lists every characteristic the employee has; History is limited to the period.
type EmployeeAnalytics struct {
	User    AnalyticsUser     `json:"user"`
	AsOf    string            `json:"asOf"`
	Current []EmployeeCurrent `json:"current"`
	History []TimelineEntry   `json:"history"`
}

type RankingEmployee struct {
	Rank       int                     `json:"rank"`
	User       UserRef                 `json:"user"`
	Department *AnalyticsDepartmentRef `json:"department"`
	Score      float64                 `json:"score"`
	MeasuredAt time.Time               `json:"measuredAt"`
}

type RankingEmployees struct {
	Total int               `json:"total"`
	Items []RankingEmployee `json:"items"`
}

// RankingDepartment: departments below MinSample are omitted, so Value is always set.
type RankingDepartment struct {
	Rank       int                     `json:"rank"`
	Department *AnalyticsDepartmentRef `json:"department"`
	Value      float64                 `json:"value"`
	Employees  int                     `json:"employees"`
	Attempts   int                     `json:"attempts"`
}

// RankingBlock: Insufficient (company n < MinSample) means no employee items.
type RankingBlock struct {
	Characteristic CharacteristicRef   `json:"characteristic"`
	Insufficient   bool                `json:"insufficient"`
	Employees      RankingEmployees    `json:"employees"`
	Departments    []RankingDepartment `json:"departments"`
}

// RankingResponse: GET /api/analytics/ranking, one block per characteristic.
type RankingResponse struct {
	AsOf      string         `json:"asOf"`
	MinSample int            `json:"minSample"`
	Blocks    []RankingBlock `json:"blocks"`
}

type ChangePoint struct {
	Score float64   `json:"score"`
	At    time.Time `json:"at"`
}

type Change struct {
	User             UserRef                 `json:"user"`
	Department       *AnalyticsDepartmentRef `json:"department"`
	CharacteristicID int64                   `json:"characteristicId"`
	From             ChangePoint             `json:"from"`
	To               ChangePoint             `json:"to"`
	Delta            float64                 `json:"delta"`
}

// ChangesResponse: GET /api/analytics/changes, sorted by |delta| descending.
type ChangesResponse struct {
	Threshold int      `json:"threshold"`
	Total     int      `json:"total"`
	Items     []Change `json:"items"`
}
