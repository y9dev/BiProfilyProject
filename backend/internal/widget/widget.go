// Package widget renders the embeddable employee skills card as a standalone HTML
// fragment: one root element, scoped inline <style>, inline SVG only, no scripts and
// no external assets. Render is pure (Data in, HTML out); all dynamic text goes
// through html/template auto-escaping.
package widget

import (
	"bytes"
	_ "embed"
	"html/template"
	"strings"

	"bigroup-backend/internal/avatar"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

//go:embed widget.css
var css string

//go:embed widget.html.tmpl
var tmplSrc string

var tmpl = template.Must(template.New("widget").Parse(tmplSrc))

// Axis is one characteristic on the radar. Core axes (default characteristics) get a
// palette colour and a legend row; extras are gray.
type Axis struct {
	Label string
	Value float64 // 0..100
	Core  bool
}

// Data is everything Render needs.
type Data struct {
	Lang       string // "ru" (default) or "kk"
	Name       string
	JobTitle   string
	Department string
	// Avatar is the triple to draw; nil renders a neutral placeholder.
	Avatar *model.Avatar
	Axes   []Axis
}

type texts struct{ Title, Empty, Avatar string }

var i18n = map[string]texts{
	"ru": {"Профиль навыков", "Пока нет результатов", "Аватар"},
	"kk": {"Дағдылар профилі", "Әзірге нәтижелер жоқ", "Аватар"},
}

// NormLang maps any input to a supported language (default ru).
func NormLang(l string) string {
	if _, ok := i18n[l]; ok {
		return l
	}
	return "ru"
}

// FromProfile builds Data from the user and the profile means. A characteristic is
// core when it carries nameI18n (set for default characteristics only).
func FromProfile(u model.User, p dto.Profile, lang string) Data {
	lang = NormLang(lang)
	d := Data{Lang: lang, Name: u.FullName, JobTitle: u.JobTitle, Avatar: u.Avatar}
	if u.DepartmentName != nil {
		d.Department = *u.DepartmentName
	}
	for _, c := range p.Characteristics {
		label := c.Name
		if l := c.NameI18n[lang]; l != "" {
			label = l
		}
		d.Axes = append(d.Axes, Axis{Label: label, Value: c.Mean, Core: c.NameI18n != nil})
	}
	return d
}

type viewAxis struct {
	Label  string
	Pct    int
	Class  string // bip-w__c0..3 for core, bip-w__cx for extras
	Core   bool
	Dot    Point
	HasDot bool
}

type view struct {
	CSS        template.CSS
	Lang       string
	T          texts
	Name       string
	JobTitle   string
	Department string
	Avatar     template.HTML
	Initial    string
	HasRadar   bool
	Rings      []string
	Spokes     []Point
	Shape      string
	Axes       []viewAxis
	Core       []viewAxis
	Empty      bool
	RadarLabel string
}

// Render produces the widget fragment. The only template.HTML inputs are the CSS and
// the avatar SVG, both generated server-side from catalog-validated ids.
func Render(d Data) (template.HTML, error) {
	d.Lang = NormLang(d.Lang)
	v := view{CSS: template.CSS(css), Lang: d.Lang, T: i18n[d.Lang], Name: d.Name,
		JobTitle: d.JobTitle, Department: d.Department, Empty: true}

	if r := []rune(strings.TrimSpace(d.Name)); len(r) > 0 {
		v.Initial = strings.ToUpper(string(r[0]))
	}
	if d.Avatar != nil && avatar.Valid(*d.Avatar) {
		v.Avatar = template.HTML(avatar.RenderSVG(d.Avatar.Universe, d.Avatar.Gender, d.Avatar.Style, 160, "bip-w-av"))
	}

	n := len(d.Axes)
	vals := make([]float64, n)
	coreIdx := 0
	for i, a := range d.Axes {
		vals[i] = clampPct(a.Value)
		if vals[i] > 0 {
			v.Empty = false
		}
		va := viewAxis{Label: a.Label, Pct: int(vals[i] + 0.5), Class: "bip-w__cx", Core: a.Core}
		if a.Core {
			va.Class = "bip-w__c" + string(rune('0'+coreIdx%4))
			coreIdx++
		}
		if n >= minRadarAxes {
			va.Dot, va.HasDot = AxisPoint(i, n, vals[i]), true
		}
		v.Axes = append(v.Axes, va)
		if a.Core {
			v.Core = append(v.Core, va)
		}
	}
	if n >= minRadarAxes {
		v.HasRadar = true
		for _, lvl := range []float64{25, 50, 75, 100} {
			v.Rings = append(v.Rings, ring(n, lvl))
		}
		for i := 0; i < n; i++ {
			v.Spokes = append(v.Spokes, AxisPoint(i, n, 100))
		}
		v.Shape = Polygon(vals)
		v.RadarLabel = v.T.Title
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
