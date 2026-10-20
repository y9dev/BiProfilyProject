package assessments

import (
	"strings"
	"testing"
	"time"

	"bigroup-backend/internal/httpapi/dto"
)

var known = map[int64]bool{1: true, 2: true, 3: true}

func eff(c int64, level string, dir int) dto.Effect {
	return dto.Effect{CharacteristicID: c, Level: level, Direction: dir}
}

func opt(text string, e ...dto.Effect) dto.CreateOptionRequest {
	if e == nil {
		e = []dto.Effect{}
	}
	return dto.CreateOptionRequest{Text: text, Effects: e}
}

func q(typ string, opts ...dto.CreateOptionRequest) dto.CreateQuestionRequest {
	return dto.CreateQuestionRequest{Text: "Q", Type: typ, Options: opts}
}

func valid() dto.CreateAssessmentRequest {
	return dto.CreateAssessmentRequest{Title: "T", Questions: []dto.CreateQuestionRequest{
		q(TypeSingle, opt("a", eff(1, "high", 1)), opt("b")),
	}}
}

func hasErr(errs []dto.FieldError, path, code string) bool {
	for _, e := range errs {
		if e.Path == path && e.Code == code {
			return true
		}
	}
	return false
}

func TestValidate_OK(t *testing.T) {
	errs, warns := Validate(valid(), known, time.Now())
	if len(errs) != 0 || len(warns) != 0 {
		t.Fatalf("errs=%v warns=%v", errs, warns)
	}
}

func TestValidate_Rules(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	long := strings.Repeat("я", 201)
	cases := []struct {
		name       string
		mut        func(r *dto.CreateAssessmentRequest)
		path, code string
	}{
		{"title required", func(r *dto.CreateAssessmentRequest) { r.Title = "  " }, "title", CodeRequired},
		{"title too long", func(r *dto.CreateAssessmentRequest) { r.Title = long }, "title", CodeTooLong},
		{"description too long", func(r *dto.CreateAssessmentRequest) { r.Description = strings.Repeat("x", 4001) }, "description", CodeTooLong},
		{"past expiry", func(r *dto.CreateAssessmentRequest) { r.ExpiresAt = &past }, "expiresAt", CodePastExpiry},
		{"no questions", func(r *dto.CreateAssessmentRequest) { r.Questions = nil }, "questions", CodeRequired},
		{"question text", func(r *dto.CreateAssessmentRequest) { r.Questions[0].Text = "" }, "questions[0].text", CodeRequired},
		{"question long", func(r *dto.CreateAssessmentRequest) { r.Questions[0].Text = strings.Repeat("x", 1001) }, "questions[0].text", CodeTooLong},
		{"bad type", func(r *dto.CreateAssessmentRequest) { r.Questions[0].Type = "text" }, "questions[0].type", CodeInvalidType},
		{"one option", func(r *dto.CreateAssessmentRequest) { r.Questions[0].Options = r.Questions[0].Options[:1] }, "questions[0].options", CodeTooFewOptions},
		{"rating 11 options", func(r *dto.CreateAssessmentRequest) {
			r.Questions[0].Type = TypeRating
			r.Questions[0].Options = nil
			for i := 0; i < 11; i++ {
				r.Questions[0].Options = append(r.Questions[0].Options, opt("x"))
			}
		}, "questions[0].options", CodeTooManyOptions},
		{"option text", func(r *dto.CreateAssessmentRequest) { r.Questions[0].Options[1].Text = "" }, "questions[0].options[1].text", CodeRequired},
		{"option long", func(r *dto.CreateAssessmentRequest) { r.Questions[0].Options[1].Text = strings.Repeat("x", 501) }, "questions[0].options[1].text", CodeTooLong},
		{"unknown characteristic", func(r *dto.CreateAssessmentRequest) {
			r.Questions[0].Options[1].Effects = []dto.Effect{eff(99, "low", 1)}
		}, "questions[0].options[1].effects[0].characteristicId", CodeUnknownCharacteristic},
		{"duplicate characteristic", func(r *dto.CreateAssessmentRequest) {
			r.Questions[0].Options[0].Effects = append(r.Questions[0].Options[0].Effects, eff(1, "low", -1))
		}, "questions[0].options[0].effects[1].characteristicId", CodeDuplicateCharacteristic},
		{"bad level", func(r *dto.CreateAssessmentRequest) {
			r.Questions[0].Options[1].Effects = []dto.Effect{eff(2, "extreme", 1)}
		}, "questions[0].options[1].effects[0].level", CodeInvalidLevel},
		{"bad direction", func(r *dto.CreateAssessmentRequest) {
			r.Questions[0].Options[1].Effects = []dto.Effect{eff(2, "low", 0)}
		}, "questions[0].options[1].effects[0].direction", CodeInvalidDirection},
		{"no measurable (no effects)", func(r *dto.CreateAssessmentRequest) {
			r.Questions[0].Options[0].Effects = []dto.Effect{}
		}, "questions", CodeNoMeasurable},
		{"no measurable (only negative)", func(r *dto.CreateAssessmentRequest) {
			r.Questions[0].Options[0].Effects = []dto.Effect{eff(1, "high", -1)}
		}, "questions", CodeNoMeasurable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := valid()
			c.mut(&r)
			errs, warns := Validate(r, known, now)
			if !hasErr(errs, c.path, c.code) {
				t.Fatalf("want {%s %s}, got %v", c.path, c.code, errs)
			}
			if warns != nil {
				t.Fatalf("warnings must be empty on errors, got %v", warns)
			}
		})
	}
}

func TestValidate_RatingTwoToTenOK(t *testing.T) {
	r := valid()
	r.Questions[0].Type = TypeRating
	for n := 2; n <= 10; n++ {
		r.Questions[0].Options = []dto.CreateOptionRequest{opt("x", eff(1, "low", 1))}
		for i := 1; i < n; i++ {
			r.Questions[0].Options = append(r.Questions[0].Options, opt("y"))
		}
		if errs, _ := Validate(r, known, time.Now()); len(errs) != 0 {
			t.Fatalf("n=%d: %v", n, errs)
		}
	}
}

func warnChars(w []dto.Warning) []int64 {
	var out []int64
	for _, x := range w {
		if x.Path != "questions" || x.Code != WarnMinUnreachable {
			panic("unexpected warning shape")
		}
		out = append(out, x.CharacteristicID)
	}
	return out
}

func TestWarnings_MinUnreachable(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		qs   []dto.CreateQuestionRequest
		want []int64
	}{
		{"single with neutral option: reachable", []dto.CreateQuestionRequest{
			q(TypeSingle, opt("a", eff(1, "high", 1)), opt("b")),
		}, nil},
		{"single, all options positive: unreachable", []dto.CreateQuestionRequest{
			q(TypeSingle, opt("a", eff(1, "high", 1)), opt("b", eff(1, "low", 1))),
		}, []int64{1}},
		{"multiple, all positive: unreachable", []dto.CreateQuestionRequest{
			q(TypeMultiple, opt("a", eff(1, "high", 1)), opt("b", eff(1, "low", 1))),
		}, []int64{1}},
		{"multiple with one neutral option: reachable", []dto.CreateQuestionRequest{
			q(TypeMultiple, opt("a", eff(1, "high", 1)), opt("b")),
		}, nil},
		{"assessment-wide: a neutral question elsewhere does not rescue positive minimum, a negative one does", []dto.CreateQuestionRequest{
			q(TypeSingle, opt("a", eff(1, "high", 1)), opt("b", eff(1, "low", 1))),       // min +1
			q(TypeSingle, opt("c", eff(1, "high", 1)), opt("d", eff(1, "moderate", -1))), // min -2
		}, nil},
		{"assessment-wide: sum of minimums positive", []dto.CreateQuestionRequest{
			q(TypeSingle, opt("a", eff(1, "high", 1)), opt("b", eff(1, "moderate", 1))), // min +2
			q(TypeSingle, opt("c", eff(1, "high", 1)), opt("d", eff(1, "low", -1))),     // min -1
		}, []int64{1}},
		{"unmeasured-by-question characteristic: neutral option exists in other question only", []dto.CreateQuestionRequest{
			q(TypeSingle, opt("a", eff(1, "low", 1)), opt("b", eff(1, "low", 1))), // min +1 on c1
			q(TypeSingle, opt("c", eff(2, "high", 1)), opt("d")),                  // c2 reachable
		}, []int64{1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs, warns := Validate(dto.CreateAssessmentRequest{Title: "T", Questions: c.qs}, known, now)
			if len(errs) != 0 {
				t.Fatalf("errs: %v", errs)
			}
			got := warnChars(warns)
			if len(got) != len(c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v want %v", got, c.want)
				}
			}
		})
	}
}
