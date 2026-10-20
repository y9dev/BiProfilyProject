package dto

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Regenerate after an intentional contract change:
//
//	go test ./internal/httpapi/dto/... -update
//
// then update frontend/src/api/contract.test.mjs (and the endpoints.js parser) in the same commit.
var update = flag.Bool("update", false, "rewrite contracts/golden/*.json from the samples")

// goldenDir is <repo>/contracts/golden, shared with frontend/src/api/contract.test.mjs.
var goldenDir = filepath.Join("..", "..", "..", "..", "contracts", "golden")

var (
	t0      = time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	t1      = time.Date(2026, 10, 15, 14, 5, 0, 0, time.UTC)
	tExpire = time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC)
	i18nCT  = map[string]string{"ru": "Критическое мышление", "kk": "Сыни ойлау"}
	i18nDsc = map[string]string{"ru": "Дисциплина", "kk": "Тәртіптілік"}
	dept    = &DepartmentRef{ID: 3, Name: "Отдел продаж"}
)

func ptr[T any](v T) *T { return &v }

var sampleScores = []Score{
	{CharacteristicID: 1, Name: "Критическое мышление", NameI18n: i18nCT, Score: 66.67},
	{CharacteristicID: 3, Name: "Дисциплина", NameI18n: i18nDsc, Score: 100},
	{CharacteristicID: 5, Name: "Работа в команде", NameI18n: nil, Score: 0},
}

var sampleInvitation = Invitation{
	ID: 12, Code: "K7Q2M9XR4TBA", URL: "https://bi.example.kz/register?code=K7Q2M9XR4TBA",
	ExpiresAt: &tExpire, MaxRegistrations: 25, RegistrationsCount: 3, Status: "active",
	CreatedAt: t0, CreatedBy: UserRef{ID: 2, FullName: "Айгерим Садыкова"},
}

var effectsSample = []Effect{
	{CharacteristicID: 1, Level: "high", Direction: 1},
	{CharacteristicID: 3, Level: "low", Direction: -1},
}

// ---------- dashboard / analytics samples ----------

var (
	chCT      = CharacteristicRef{ID: 1, Name: "Критическое мышление", NameI18n: i18nCT, Core: true}
	chDsc     = CharacteristicRef{ID: 3, Name: "Дисциплина", NameI18n: i18nDsc, Core: true}
	chTeam    = CharacteristicRef{ID: 5, Name: "Работа в команде", NameI18n: nil, Core: false}
	aDept     = &AnalyticsDepartmentRef{ID: 3, Name: "Отдел продаж", Active: true}
	aDeptOff  = &AnalyticsDepartmentRef{ID: 6, Name: "Снабжение", Active: false}
	periodDay = PeriodInfo{From: "2026-10-18", To: "2026-10-20", Granularity: "day", Timezone: "Asia/Almaty"}
	daysPts   = []string{"2026-10-18", "2026-10-19", "2026-10-20"}
	vpGap     = ValuePoint{Value: nil, Employees: 0, Attempts: 0, Insufficient: true}
	vpSmall   = ValuePoint{Value: nil, Employees: 3, Attempts: 4, Insufficient: true}       // manager / group form
	vpSmallV  = ValuePoint{Value: ptr(58.3), Employees: 3, Attempts: 4, Insufficient: true} // admin company form
	vpOK      = ValuePoint{Value: ptr(61.4), Employees: 12, Attempts: 30, Insufficient: false}
	vpOK2     = ValuePoint{Value: ptr(64.2), Employees: 30, Attempts: 70, Insufficient: false}
)

var sampleTrend = TrendResponse{
	Period: periodDay, MinSample: 5, Points: daysPts,
	Series: []TrendSeries{
		{Characteristic: chCT, Values: []ValuePoint{vpGap, vpSmall, vpOK}},
		{Characteristic: chDsc, Values: []ValuePoint{vpGap, vpGap, vpSmall}},
	},
	Summary: TrendSummary{Employees: 40, MeasuredEmployees: 32, AttemptsInPeriod: 55},
}

var analyticsSamples = map[string]any{
	"dashboard_trend.json": sampleTrend,
	"analytics_trend.json": AnalyticsTrendResponse{
		TrendResponse: TrendResponse{
			Period: periodDay, MinSample: 5, Points: daysPts,
			Series: []TrendSeries{
				{Characteristic: chCT, Values: []ValuePoint{vpGap, vpSmallV, vpOK}},
				{Characteristic: chTeam, Values: []ValuePoint{vpGap, vpGap, vpSmallV}},
			},
			Summary: TrendSummary{Employees: 40, MeasuredEmployees: 32, AttemptsInPeriod: 55},
		},
		DefaultIDs: []int64{1, 3},
		Characteristics: []CharacteristicCoverage{
			{CharacteristicRef: chCT, Employees: 32, Attempts: 50, Eligible: true},
			{CharacteristicRef: chDsc, Employees: 0, Attempts: 0, Eligible: false},
			{CharacteristicRef: chTeam, Employees: 3, Attempts: 4, Eligible: false},
		},
	},
	"team_status.json": TeamStatus{
		AsOf: "2026-10-20", InactiveDays: 30, DropThreshold: 15,
		KPIs: TeamKPIs{Employees: 40, Participated: 34, NoAttempts: 6, ActiveRecently: 18, Inactive: 16, ParticipationRate: ptr(0.85)},
		Tests: []TestCompletion{
			{ID: 9, Title: "Планирование смены", Completed: 30, Employees: 40, Rate: ptr(0.75)},
		},
		Attention: Attention{
			Total: 22, Counts: AttentionCounts{NoFirstTest: 6, Inactive: 16, Drop: 3},
			Items: []AttentionItem{
				{User: UserRef{ID: 8, FullName: "Дана Ахметова"}, Department: aDeptOff, LastAttemptAt: &t0,
					Reasons: []Reason{
						{Code: "drop", CharacteristicID: ptr(int64(1)), Delta: ptr(-22.5), At: "2026-10-01"},
						{Code: "inactive", Days: ptr(41)},
					}},
				{User: UserRef{ID: 7, FullName: "Арман Нурланов"}, Department: nil, LastAttemptAt: nil,
					Reasons: []Reason{{Code: "no_first_test"}}},
			},
		},
	},
	"analytics_participation.json": ParticipationResponse{
		Period:  periodDay,
		Company: ParticipationCompany{Employees: 40, Participated: 22, Attempts: 55, Rate: ptr(0.55)},
		Departments: []ParticipationDepartment{
			{Department: aDept, Employees: 12, Participated: 7, Attempts: 15, Rate: ptr(0.5833), Insufficient: false},
			{Department: aDeptOff, Employees: 3, Participated: 2, Attempts: 2, Rate: nil, Insufficient: true},
			{Department: nil, Employees: 25, Participated: 13, Attempts: 38, Rate: ptr(0.52), Insufficient: false},
		},
		Tests: []TestCompletion{
			{ID: 9, Title: "Планирование смены", Completed: 18, Employees: 40, Rate: ptr(0.45)},
		},
	},
	"analytics_departments.json": DepartmentsResponse{
		Period: periodDay, Points: daysPts,
		Blocks: []DepartmentsBlock{{
			Characteristic: chCT,
			Company:        CompanySeries{Values: []ValuePoint{vpGap, vpOK, vpOK2}},
			Departments: []DepartmentSeries{
				{Department: aDept, Employees: 12, AttemptsInPeriod: 20, Values: []ValuePoint{vpGap, vpOK, vpOK},
					Current: DepartmentCurrent{ValuePoint: ValuePoint{Value: ptr(63.0), Employees: 12, Attempts: 25}, VsCompany: ptr(1.6)}},
				{Department: aDeptOff, Employees: 3, AttemptsInPeriod: 2, Values: []ValuePoint{vpGap, vpSmall, vpSmall},
					Current: DepartmentCurrent{ValuePoint: vpSmall, VsCompany: nil}},
				{Department: nil, Employees: 25, AttemptsInPeriod: 30, Values: []ValuePoint{vpGap, vpOK, vpOK2},
					Current: DepartmentCurrent{ValuePoint: vpOK2, VsCompany: ptr(0.0)}},
			},
		}},
	},
	"analytics_employees.json": AnalyticsEmployeeList{Total: 3, Items: []AnalyticsEmployee{
		{ID: 7, FullName: "Арман Нурланов", JobTitle: "Инженер ПТО", Department: nil, Attempts: 0, LastAttemptAt: nil, DataState: "no_data"},
		{ID: 8, FullName: "Дана Ахметова", JobTitle: "", Department: aDeptOff, Attempts: 2, LastAttemptAt: &t0, DataState: "stale"},
		{ID: 9, FullName: "Ерлан Касымов", JobTitle: "Прораб", Department: aDept, Attempts: 5, LastAttemptAt: &t1, DataState: "ok"},
	}},
	"analytics_employee.json": EmployeeAnalytics{
		User: AnalyticsUser{ID: 7, FullName: "Арман Нурланов", JobTitle: "Инженер ПТО", Department: nil, Status: "blocked", LastAttemptAt: &t1},
		AsOf: "2026-10-20",
		Current: []EmployeeCurrent{
			{Characteristic: chCT, Score: 70, MeasuredAt: t1, Company: vpOK2, Department: vpSmall},
			{Characteristic: chTeam, Score: 0, MeasuredAt: t1, Company: vpSmall, Department: vpSmall},
		},
		History: []TimelineEntry{
			{AttemptID: 41, AssessmentTitle: "Планирование смены", CompletedAt: t1, Scores: sampleScores},
		},
	},
	"analytics_ranking.json": RankingResponse{AsOf: "2026-10-20", MinSample: 5, Blocks: []RankingBlock{
		{Characteristic: chCT, Insufficient: false,
			Employees: RankingEmployees{Total: 30, Items: []RankingEmployee{
				{Rank: 1, User: UserRef{ID: 9, FullName: "Ерлан Касымов"}, Department: aDept, Score: 91.7, MeasuredAt: t1},
				{Rank: 1, User: UserRef{ID: 8, FullName: "Дана Ахметова"}, Department: aDeptOff, Score: 91.7, MeasuredAt: t0},
				{Rank: 3, User: UserRef{ID: 7, FullName: "Арман Нурланов"}, Department: nil, Score: 70, MeasuredAt: t1},
			}},
			Departments: []RankingDepartment{
				{Rank: 1, Department: aDept, Value: 72.4, Employees: 12, Attempts: 25},
				{Rank: 2, Department: nil, Value: 61.0, Employees: 18, Attempts: 45},
			}},
		{Characteristic: chTeam, Insufficient: true,
			Employees:   RankingEmployees{Total: 3, Items: []RankingEmployee{}},
			Departments: []RankingDepartment{}},
	}},
	"analytics_changes.json": ChangesResponse{Threshold: 15, Total: 2, Items: []Change{
		{User: UserRef{ID: 8, FullName: "Дана Ахметова"}, Department: aDeptOff, CharacteristicID: 1,
			From: ChangePoint{Score: 80, At: t0}, To: ChangePoint{Score: 57.5, At: t1}, Delta: -22.5},
		{User: UserRef{ID: 7, FullName: "Арман Нурланов"}, Department: nil, CharacteristicID: 5,
			From: ChangePoint{Score: 50, At: t0}, To: ChangePoint{Score: 66.67, At: t1}, Delta: 16.67},
	}},
}

func init() {
	for name, v := range analyticsSamples {
		samples[name] = v
	}
}

// samples maps each golden file to the DTO value it freezes.
var samples = map[string]any{
	"me.json": Me{
		ID: 7, Login: "a.nurlanov", FullName: "Арман Нурланов", JobTitle: "Инженер ПТО",
		Department: dept, Role: "employee",
		Avatar: &Avatar{Universe: "anime", Gender: "male", Style: "tech"}, ProfileComplete: true,
	},
	"register_options.json": RegisterOptions{
		Valid: true, Departments: []DepartmentRef{{ID: 3, Name: "Отдел продаж"}, {ID: 4, Name: "Производство"}},
	},
	"register_options_invalid.json": RegisterOptions{Valid: false, Reason: "invite_invalid"},
	"avatar_catalog.json": AvatarCatalog{
		Universes: []string{"movies", "anime", "animals", "sports", "fantasy"},
		Genders:   []string{"male", "female"},
		Styles:    []string{"vibrant", "formal", "calm", "tech"},
	},
	"profile.json": Profile{
		Characteristics: []ProfileCharacteristic{
			{ID: 1, Name: "Критическое мышление", NameI18n: i18nCT, Mean: 72.5, Count: 2},
			{ID: 3, Name: "Дисциплина", NameI18n: i18nDsc, Mean: 100, Count: 1},
			{ID: 5, Name: "Работа в команде", NameI18n: nil, Mean: 0, Count: 1},
		},
		Timeline: []TimelineEntry{
			{AttemptID: 40, AssessmentTitle: "Решения на объекте", CompletedAt: t0, Scores: sampleScores[:1]},
			{AttemptID: 41, AssessmentTitle: "Планирование смены", CompletedAt: t1, Scores: sampleScores},
		},
		Stats: ProfileStats{Completed: 2, Available: 3},
	},
	"assessments_list.json": []AssessmentListItem{
		{ID: 9, Title: "Планирование смены", Description: "Ситуации на стройплощадке", ExpiresAt: nil,
			QuestionCount: 8, Completed: true, AttemptID: ptr(int64(41))},
		{ID: 10, Title: "Работа с подрядчиком", Description: "", ExpiresAt: &tExpire,
			QuestionCount: 5, Completed: false},
	},
	"take_view.json": TakeView{
		ID: 10, Title: "Работа с подрядчиком", Description: "",
		Questions: []TakeQuestion{
			{ID: 100, Position: 0, Text: "Подрядчик срывает срок. Что вы сделаете?", Type: "single", Options: []TakeOption{
				{ID: 1000, Position: 0, Text: "Позвоню и выясню причину"},
				{ID: 1001, Position: 1, Text: "Сразу сообщу руководителю"},
			}},
			{ID: 101, Position: 1, Text: "Что вы проверяете перед приёмкой?", Type: "multiple", Options: []TakeOption{
				{ID: 1002, Position: 0, Text: "Акты скрытых работ"},
				{ID: 1003, Position: 1, Text: "Исполнительную документацию"},
				{ID: 1004, Position: 2, Text: "Ничего, доверяю подрядчику"},
			}},
			{ID: 102, Position: 2, Text: "Насколько вы уверены в графике?", Type: "rating", Options: []TakeOption{
				{ID: 1005, Position: 0, Text: "1"}, {ID: 1006, Position: 1, Text: "2"}, {ID: 1007, Position: 2, Text: "3"},
			}},
		},
	},
	"submit_request.json": SubmitRequest{Answers: []SubmitAnswer{
		{QuestionID: 100, OptionIDs: []int64{1000}},
		{QuestionID: 101, OptionIDs: []int64{1002, 1003}},
		{QuestionID: 102, OptionIDs: []int64{1007}},
	}},
	"submit_result.json":  SubmitResult{AttemptID: 42, Scores: sampleScores},
	"attempt_result.json": AttemptResult{AttemptID: 42, AssessmentID: 10, AssessmentTitle: "Работа с подрядчиком", CompletedAt: t1, Scores: sampleScores},
	"characteristics.json": []Characteristic{
		{ID: 1, Name: "Критическое мышление", NameI18n: i18nCT, IsDefault: true, InUse: true},
		{ID: 5, Name: "Работа в команде", NameI18n: nil, IsDefault: false, InUse: false},
	},
	"departments.json": []Department{
		{ID: 3, Name: "Отдел продаж", Active: true, UserCount: 12},
		{ID: 6, Name: "Снабжение", Active: false, UserCount: 2},
	},
	"department_delete.json": DepartmentDeleteResult{Deactivated: true},
	"invitation.json":        sampleInvitation,
	"invitations_list.json": []Invitation{sampleInvitation, {
		ID: 13, Code: "P3WZ8N2HC5VD", URL: "https://bi.example.kz/register?code=P3WZ8N2HC5VD",
		ExpiresAt: nil, MaxRegistrations: 1, RegistrationsCount: 1, Status: "full",
		CreatedAt: t1, CreatedBy: UserRef{ID: 2, FullName: "Айгерим Садыкова"},
	}},
	"registrations.json": []Registrant{
		{FullName: "Арман Нурланов", JobTitle: "Инженер ПТО", Department: dept},
		{FullName: "Дана Ахметова", JobTitle: "", Department: nil},
	},
	"manage_assessments_list.json": []ManageAssessmentListItem{
		{ID: 9, Title: "Планирование смены", Description: "Ситуации на стройплощадке", Hidden: false, ExpiresAt: nil,
			Status: "active", QuestionCount: 8, AttemptCount: 14, CreatedAt: t0, UpdatedAt: t1},
		{ID: 11, Title: "Черновик опроса", Description: "", Hidden: true, ExpiresAt: &tExpire,
			Status: "hidden", QuestionCount: 3, AttemptCount: 0, CreatedAt: t1, UpdatedAt: t1},
	},
	"manage_assessment.json": ManageAssessment{
		ID: 10, Title: "Работа с подрядчиком", Description: "", Hidden: false, ExpiresAt: &tExpire,
		Status: "active", AttemptCount: 1, CreatedAt: t0, UpdatedAt: t1,
		Questions: []ManageQuestion{
			{ID: 100, Position: 0, Text: "Подрядчик срывает срок. Что вы сделаете?", Type: "single", Options: []ManageOption{
				{ID: 1000, Position: 0, Text: "Позвоню и выясню причину", Effects: effectsSample},
				{ID: 1001, Position: 1, Text: "Сразу сообщу руководителю", Effects: []Effect{}},
			}},
		},
	},
	"create_assessment_request.json": CreateAssessmentRequest{
		Title: "Работа с подрядчиком", Description: "", ExpiresAt: &tExpire, Hidden: false,
		Questions: []CreateQuestionRequest{
			{Text: "Подрядчик срывает срок. Что вы сделаете?", Type: "single", Options: []CreateOptionRequest{
				{Text: "Позвоню и выясню причину", Effects: effectsSample},
				{Text: "Сразу сообщу руководителю", Effects: []Effect{}},
			}},
			{Text: "Насколько вы уверены в графике?", Type: "rating", Options: []CreateOptionRequest{
				{Text: "1", Effects: []Effect{}},
				{Text: "2", Effects: []Effect{{CharacteristicID: 2, Level: "moderate", Direction: 1}}},
			}},
		},
	},
	"create_assessment_result.json": CreateAssessmentResult{ID: 10, Warnings: []Warning{
		{Path: "questions", Code: "min_unreachable", CharacteristicID: 2},
	}},
	"admin_users.json": AdminUserList{Total: 2, Items: []AdminUser{
		{ID: 1, Login: "admin", FullName: "Администратор", JobTitle: "", Department: nil, Role: "admin", Status: "active", CreatedAt: t0},
		{ID: 7, Login: "a.nurlanov", FullName: "Арман Нурланов", JobTitle: "Инженер ПТО", Department: dept, Role: "employee", Status: "blocked", CreatedAt: t1},
	}},
	"error_validation.json": Error{Error: "validation", Errors: []FieldError{
		{Path: "questions[2].options[1].effects[0].characteristicId", Code: "unknown_characteristic"},
		{Path: "title", Code: "required"},
	}},
	"error.json": Error{Error: "avatar_required"},
}

func marshalGolden(v any) ([]byte, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func TestGolden(t *testing.T) {
	names := make([]string, 0, len(samples))
	for n := range samples {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got, err := marshalGolden(samples[name])
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			path := filepath.Join(goldenDir, name)
			if *update {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")) // tolerate CRLF checkouts
			if !bytes.Equal(got, want) {
				t.Fatalf("%s drifted from the DTO.\n--- golden\n%s\n--- dto\n%s", name, want, got)
			}
		})
	}
}

// TestGoldenHasNoOrphans: every fixture in contracts/golden is backed by a DTO sample.
func TestGoldenHasNoOrphans(t *testing.T) {
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			if _, ok := samples[e.Name()]; !ok {
				t.Errorf("%s has no DTO sample in golden_test.go", e.Name())
			}
		}
	}
}

// TestTakeViewHasNoEffects: the employee take-view cannot carry scoring data (AC-12).
func TestTakeViewHasNoEffects(t *testing.T) {
	raw, err := marshalGolden(samples["take_view.json"])
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"effects"`, `"level"`, `"direction"`, `"characteristicId"`} {
		if bytes.Contains(raw, []byte(key)) {
			t.Fatalf("take view leaks %s", key)
		}
	}
}

// TestRequestsRoundTrip: frozen request bodies decode into their DTOs without unknown fields.
func TestRequestsRoundTrip(t *testing.T) {
	for name, target := range map[string]any{
		"submit_request.json":            &SubmitRequest{},
		"create_assessment_request.json": &CreateAssessmentRequest{},
	} {
		raw, err := os.ReadFile(filepath.Join(goldenDir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(target); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
