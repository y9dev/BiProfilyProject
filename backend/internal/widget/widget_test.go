package widget

import (
	"math"
	"strings"
	"testing"

	"bigroup-backend/internal/model"
)

func axes(n int, v float64) []Axis {
	out := make([]Axis, n)
	for i := range out {
		out[i] = Axis{Label: "A", Value: v, Core: i < 4}
	}
	return out
}

func render(t *testing.T, d Data) string {
	t.Helper()
	h, err := Render(d)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return string(h)
}

func TestEscapesDynamicText(t *testing.T) {
	xss := `<script>alert(1)</script>`
	out := render(t, Data{Name: xss, JobTitle: xss, Department: xss,
		Axes: []Axis{{Label: xss, Value: 50, Core: true}, {Label: "b", Value: 1}, {Label: "c", Value: 1}}})
	if strings.Contains(out, "<script") {
		t.Fatalf("unescaped script in output")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("expected escaped text")
	}
}

func TestStandalone(t *testing.T) {
	out := render(t, Data{Name: "Аня", Axes: axes(4, 40)})
	for _, bad := range []string{"<script", "http://", "https://", "<link", "@import", "<img"} {
		if strings.Contains(out, bad) {
			t.Errorf("output contains %q", bad)
		}
	}
	if !strings.HasPrefix(out, `<div class="bip-w"`) {
		t.Errorf("unexpected root: %.40s", out)
	}
}

func TestEmptyStateAndLang(t *testing.T) {
	ru := render(t, Data{Name: "X", Axes: axes(4, 0)})
	if !strings.Contains(ru, "Пока нет результатов") || strings.Contains(ru, "<polygon class=\"bip-w__shape\"") {
		t.Errorf("ru empty state wrong")
	}
	kk := render(t, Data{Name: "X", Lang: "kk", Axes: axes(4, 0)})
	if !strings.Contains(kk, "Әзірге нәтижелер жоқ") || !strings.Contains(kk, `lang="kk"`) {
		t.Errorf("kk empty state wrong")
	}
	if NormLang("xx") != "ru" {
		t.Errorf("unknown lang must fall back to ru")
	}
}

func TestPaletteAndLegend(t *testing.T) {
	out := render(t, Data{Name: "X", Axes: axes(6, 50)})
	for _, c := range []string{"bip-w__c0", "bip-w__c1", "bip-w__c2", "bip-w__c3", "bip-w__cx"} {
		if !strings.Contains(out, c) {
			t.Errorf("missing %s", c)
		}
	}
	if n := strings.Count(out, `<span class="v">50%</span>`); n != 4 {
		t.Errorf("legend rows = %d, want 4 core", n)
	}
}

func TestNeutralAvatarAndSVGAvatar(t *testing.T) {
	out := render(t, Data{Name: "Борис", Axes: axes(4, 1)})
	if !strings.Contains(out, `class="bip-w__initial"`) || !strings.Contains(out, ">Б<") {
		t.Errorf("neutral avatar missing")
	}
	out = render(t, Data{Name: "Борис", Axes: axes(4, 1), Avatar: &model.Avatar{Universe: "movies", Gender: "male", Style: "calm"}})
	if strings.Contains(out, `class="bip-w__initial"`) || !strings.Contains(out, "<svg") {
		t.Errorf("avatar svg missing")
	}
	// Out-of-catalog triple falls back to neutral.
	out = render(t, Data{Name: "B", Axes: axes(4, 1), Avatar: &model.Avatar{Universe: "x", Gender: "y", Style: "z"}})
	if !strings.Contains(out, `class="bip-w__initial"`) {
		t.Errorf("invalid avatar must be neutral")
	}
}

func TestRadarGeometryAxes(t *testing.T) {
	for n := 0; n <= 12; n++ {
		out := render(t, Data{Name: "X", Axes: axes(n, 60)})
		hasRadar := strings.Contains(out, `class="bip-w__shape"`)
		if want := n >= 3; hasRadar != want {
			t.Errorf("n=%d radar=%v want %v", n, hasRadar, want)
		}
		if n >= 3 {
			if got := strings.Count(out, `<line class="bip-w__spoke"`); got != n {
				t.Errorf("n=%d spokes=%d", n, got)
			}
			if got := strings.Count(out, "<circle"); got != n {
				t.Errorf("n=%d dots=%d", n, got)
			}
		}
	}
}

func TestAxisPoint(t *testing.T) {
	p := AxisPoint(0, 4, 100)
	if math.Abs(p.X-cx) > 1e-9 || math.Abs(p.Y-(cy-radius)) > 1e-9 {
		t.Errorf("axis 0 should point up, got %v", p)
	}
	p = AxisPoint(1, 4, 100)
	if math.Abs(p.X-(cx+radius)) > 1e-9 || math.Abs(p.Y-cy) > 1e-9 {
		t.Errorf("axis 1 of 4 should point right, got %v", p)
	}
	if q := AxisPoint(2, 5, 250); math.Hypot(q.X-cx, q.Y-cy) > radius+1e-9 {
		t.Errorf("value must clamp to 100")
	}
	if q := AxisPoint(0, 3, math.NaN()); q.X != cx || q.Y != cy {
		t.Errorf("NaN should map to centre")
	}
	for n := 3; n <= 12; n++ {
		for i := 0; i < n; i++ {
			q := AxisPoint(i, n, 100)
			if math.Abs(math.Hypot(q.X-cx, q.Y-cy)-radius) > 1e-9 {
				t.Errorf("n=%d i=%d not on ring", n, i)
			}
		}
	}
}
