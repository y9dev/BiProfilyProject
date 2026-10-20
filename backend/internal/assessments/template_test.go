package assessments

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/scoring"
)

// The "construction staff" template is owned by the frontend (wizard/templates). This test feeds
// the very same JSON through the server's own validation and scoring, so the weights stored in the
// template are checked against the trusted backend, not against a client-side copy.
const templatePath = "../../../frontend/src/features/manager/wizard/templates/construction-staff.json"

type tplFile struct {
	Traits    []string `json:"traits"`
	Questions []struct {
		Title   string `json:"title"`
		Text    string `json:"text"`
		Type    string `json:"type"`
		Options []struct {
			Text    string         `json:"text"`
			Effects map[string]int `json:"effects"`
		} `json:"options"`
	} `json:"questions"`
}

func loadTemplate(t *testing.T) tplFile {
	t.Helper()
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	var f tplFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// request is what the wizard would POST: trait i has characteristic id i+1; +n/-n map to
// direction ±1 and level low/moderate (weights 1 and 2).
func (f tplFile) request() dto.CreateAssessmentRequest {
	ids := map[string]int64{}
	for i, name := range f.Traits {
		ids[name] = int64(i + 1)
	}
	req := dto.CreateAssessmentRequest{Title: "t"}
	for _, q := range f.Questions {
		cq := dto.CreateQuestionRequest{Text: q.Title + "\n" + q.Text, Type: q.Type}
		for _, o := range q.Options {
			co := dto.CreateOptionRequest{Text: o.Text, Effects: []dto.Effect{}}
			for name, n := range o.Effects {
				level, dir := "low", 1
				if n < 0 {
					dir, n = -1, -n
				}
				if n == 2 {
					level = "moderate"
				}
				co.Effects = append(co.Effects, dto.Effect{CharacteristicID: ids[name], Level: level, Direction: dir})
			}
			cq.Options = append(cq.Options, co)
		}
		req.Questions = append(req.Questions, cq)
	}
	return req
}

func (f tplFile) scoringQuestions() []scoring.Question {
	req := f.request()
	var qs []scoring.Question
	for qi, q := range req.Questions {
		sq := scoring.Question{ID: int64(qi + 1), Type: q.Type}
		for oi, o := range q.Options {
			so := scoring.Option{ID: int64(qi*10 + oi + 1), Effects: map[int64]int{}}
			for _, e := range o.Effects {
				so.Effects[e.CharacteristicID] = scoring.Signed(e.Direction, e.Level)
			}
			sq.Options = append(sq.Options, so)
		}
		qs = append(qs, sq)
	}
	return qs
}

func TestTemplateShape(t *testing.T) {
	f := loadTemplate(t)
	if len(f.Traits) != 4 || len(f.Questions) != 12 {
		t.Fatalf("traits=%d questions=%d", len(f.Traits), len(f.Questions))
	}
	options := 0
	for i, q := range f.Questions {
		if q.Type != "single" || len(q.Options) != 4 {
			t.Fatalf("question %d: type=%s options=%d", i+1, q.Type, len(q.Options))
		}
		options += len(q.Options)
	}
	if options != 48 {
		t.Fatalf("options=%d", options)
	}
}

func TestTemplatePassesServerValidation(t *testing.T) {
	f := loadTemplate(t)
	known := map[int64]bool{1: true, 2: true, 3: true, 4: true}
	errs, warns := Validate(f.request(), known, time.Now())
	if len(errs) != 0 {
		t.Fatalf("validation errors: %+v", errs)
	}
	// Documented property of the supplied key: every answer to some questions raises the trait, so
	// 0% is unreachable for critical thinking (1) and discipline (3). Non-blocking by design.
	got := map[int64]bool{}
	for _, w := range warns {
		if w.Code == "min_unreachable" {
			got[w.CharacteristicID] = true
		}
	}
	if len(got) != 2 || !got[1] || !got[3] {
		t.Fatalf("min_unreachable warnings = %v, want traits 1 and 3", got)
	}
}

func TestTemplateTraitMaximaAndTotals(t *testing.T) {
	qs := loadTemplate(t).scoringQuestions()

	// Hand-computed from the supplied matrix: per-trait maxima (sum of each question's best option).
	maxes := scoring.MaxTotals(qs)
	for id, want := range map[int64]int{1: 22, 2: 19, 3: 24, 4: 20} {
		if maxes[id] != want {
			t.Errorf("MAX(trait %d) = %d, want %d", id, maxes[id], want)
		}
	}

	pick := func(letter int) map[int64][]int64 {
		a := map[int64][]int64{}
		for _, q := range qs {
			a[q.ID] = []int64{q.Options[letter].ID}
		}
		return a
	}
	cases := []struct {
		name   string
		letter int
		raw    [4]int
		score  [4]float64
	}{
		// raw = sum over the 12 questions of the chosen option's weights (hand-computed).
		{"all A", 0, [4]int{18, 14, 18, 12}, [4]float64{81.82, 73.68, 75, 60}},
		{"all B", 1, [4]int{10, 1, 18, -6}, [4]float64{45.45, 5.26, 75, 0}}, // negative raw clamps to 0
		{"all C", 2, [4]int{16, 6, 11, 12}, [4]float64{72.73, 31.58, 45.83, 60}},
		{"all D", 3, [4]int{13, 2, 9, 7}, [4]float64{59.09, 10.53, 37.5, 35}},
	}
	for _, c := range cases {
		res := scoring.Compute(qs, pick(c.letter))
		if len(res) != 4 {
			t.Fatalf("%s: %d results", c.name, len(res))
		}
		for i, r := range res {
			if r.Raw != c.raw[i] || r.Score != c.score[i] {
				t.Errorf("%s trait %d: raw=%d score=%v, want raw=%d score=%v", c.name, i+1, r.Raw, r.Score, c.raw[i], c.score[i])
			}
		}
	}
}

func TestNormalizationEdgeCases(t *testing.T) {
	if scoring.Normalize(5, 0) != 0 || scoring.Normalize(0, 0) != 0 || scoring.Normalize(-3, 0) != 0 {
		t.Error("max <= 0 must give 0 without dividing by zero")
	}
	if scoring.Normalize(99, 10) != 100 || scoring.Normalize(-4, 10) != 0 {
		t.Error("scores are clamped to 0..100")
	}
}

func TestParticipantViewHasNoScoringWeights(t *testing.T) {
	view := dto.TakeView{ID: 1, Title: "t", Questions: []dto.TakeQuestion{{ID: 1, Options: []dto.TakeOption{{ID: 1, Text: "a"}}}}}
	b, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"effect", "weight", "direction", "level", "characteristic"} {
		if strings.Contains(strings.ToLower(string(b)), banned) {
			t.Errorf("take view JSON exposes %q: %s", banned, b)
		}
	}
}
