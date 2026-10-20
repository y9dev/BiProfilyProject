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
