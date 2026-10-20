package avatar

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func decodeAll(t *testing.T, svg string) (ids []string) {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(svg))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return ids
		}
		if err != nil {
			t.Fatalf("malformed xml: %v\n%s", err, svg)
		}
		if se, ok := tok.(xml.StartElement); ok {
			for _, a := range se.Attr {
				if a.Name.Local == "id" {
					ids = append(ids, a.Value)
				}
			}
		}
	}
}

func TestRenderSVGAllCombinations(t *testing.T) {
	for _, u := range Universes {
		for _, g := range Genders {
			for _, s := range Styles {
				svg := RenderSVG(u, g, s, 96, "w1-")
				if !strings.HasPrefix(svg, "<svg ") || !strings.HasSuffix(svg, "</svg>") {
					t.Fatalf("%s.%s.%s: not an svg root", u, g, s)
				}
				if !strings.Contains(svg, `width="96"`) || !strings.Contains(svg, `viewBox="0 0 64 64"`) {
					t.Fatalf("%s.%s.%s: missing size/viewBox", u, g, s)
				}
				ids := decodeAll(t, svg)
				if len(ids) == 0 {
					t.Fatalf("%s.%s.%s: expected at least one id", u, g, s)
				}
				for _, id := range ids {
					if !strings.HasPrefix(id, "w1-") {
						t.Errorf("id %q lacks prefix", id)
					}
				}
				if svg != RenderSVG(u, g, s, 96, "w1-") {
					t.Errorf("%s.%s.%s: not deterministic", u, g, s)
				}
			}
		}
	}
}

func TestRenderSVGFallbackAndSanitizing(t *testing.T) {
	def := RenderSVG("movies", "male", "vibrant", 48, "p")
	if got := RenderSVG("nope", "x", "<script>", 48, "p"); got != def {
		t.Error("unknown ids must fall back to the default avatar")
	}
	svg := RenderSVG("", "", "", 0, `"><script>alert(1)</script>`)
	decodeAll(t, svg)
	if strings.Contains(svg, "<script") || !strings.Contains(svg, `width="48"`) {
		t.Errorf("prefix not sanitized or size default missing: %s", svg)
	}
}
