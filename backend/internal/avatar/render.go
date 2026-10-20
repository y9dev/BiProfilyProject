package avatar

// Server-side port of frontend/src/avatar/{catalog.js,Avatar.jsx}. The output is meant to be
// embedded into html/template as already-safe markup, so every dynamic value is either a
// constant from the tables below or a number; the only caller-supplied string (idPrefix) is
// reduced to [A-Za-z0-9_-]. Unknown ids fall back to the default avatar.

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type palette struct {
	bg, bg2, cloth, cloth2, accent, metal, hair, hairAlt, eye, ink, jersey, jersey2, blush string
}

type body struct{ skin, shade, light, inner string }

var palettes = map[string]palette{
	"vibrant": {"#FFC21A", "#FF7A1A", "#2F6BFF", "#FFFFFF", "#FF3D71", "#FFD23F", "#2A1A14", "#FF3D71", "#1E88FF", "#1B1230", "#E8332D", "#FFFFFF", "#FF8FA8"},
	"formal":  {"#CBD3DD", "#B3BECB", "#28323F", "#FFFFFF", "#8E2433", "#C9A23E", "#2B2623", "#6B7685", "#3B4A5C", "#1B1F26", "#1F3A8A", "#FFFFFF", "#E7A9A0"},
	"calm":    {"#D8EBE2", "#BFD9E6", "#9DBBD0", "#F4EFE6", "#E8A5A0", "#E9C27A", "#8B6F5C", "#C9B8E8", "#6C9BCB", "#4A4350", "#F2A7A0", "#FFFFFF", "#F2B3AE"},
	"tech":    {"#0C1626", "#17304D", "#1B2F4A", "#2D4B73", "#35F0DC", "#35F0DC", "#5A7092", "#35F0DC", "#35F0DC", "#050B14", "#1B2F4A", "#35F0DC", "#3A6F8F"},
}

var (
	skin     = map[string]body{"male": {skin: "#EDB98F", shade: "#D99C74"}, "female": {skin: "#F4CBA8", shade: "#E1AB86"}}
	elfSkin  = map[string]body{"male": {skin: "#F3D5BC", shade: "#DDB698"}, "female": {skin: "#F8E0CC", shade: "#E4C0A4"}}
	fur      = map[string]body{"male": {"#E07A2D", "#B85C1B", "#FFF4E6", "#3A2418"}, "female": {"#EFD2AC", "#D2AE82", "#FFFDF8", "#F3A6B0"}}
	hairKind = map[string][2]string{ // male, female
		"movies": {"swept", "wave"}, "anime": {"spiky", "anime_long"}, "animals": {"none", "none"},
		"sports": {"short", "ponytail"}, "fantasy": {"hood", "long_crown"},
	}
	clothingOf = map[string]string{"movies": "style", "anime": "style", "animals": "style", "sports": "jersey", "fantasy": "robe"}
	motifOf    = map[string]string{"movies": "clapper", "anime": "sparkles", "animals": "paw", "sports": "ball", "fantasy": "stars"}
	earsOf     = map[string]string{"movies": "human", "anime": "human", "animals": "animal", "sports": "human", "fantasy": "elf"}
	mouthOf    = map[string]string{"vibrant": "grin", "formal": "slight", "calm": "soft", "tech": "flat"}
	backdropOf = map[string]string{"vibrant": "burst", "formal": "panel", "calm": "blobs", "tech": "circuit"}
)

type spec struct {
	universe, gender, style string
	p                       palette
	b                       body
	hairShape, hairColor    string
	clothing, motif, ears   string
	anime                   bool // eyes === 'anime'
	mouth, backdrop, torso  string
	visor, blush            bool
}

func pick(list []string, v, fallback string) string {
	if slices.Contains(list, v) {
		return v
	}
	return fallback
}

func newSpec(universe, gender, style string) spec {
	universe = pick(Universes, universe, "movies")
	gender = pick(Genders, gender, "male")
	style = pick(Styles, style, "vibrant")
	p := palettes[style]
	animal := universe == "animals"
	fantasy := universe == "fantasy"
	b := skin[gender]
	if animal {
		b = fur[gender]
	} else if fantasy {
		b = elfSkin[gender]
	}
	gi := 0
	if gender == "female" {
		gi = 1
	}
	hc := p.hair
	if universe == "anime" || (fantasy && gender == "female") {
		hc = p.hairAlt
	}
	torso := "narrow"
	if gender == "male" {
		torso = "broad"
	}
	return spec{
		universe: universe, gender: gender, style: style, p: p, b: b,
		hairShape: hairKind[universe][gi], hairColor: hc,
		clothing: clothingOf[universe], motif: motifOf[universe], ears: earsOf[universe],
		anime: universe == "anime", mouth: mouthOf[style], backdrop: backdropOf[style], torso: torso,
		visor: style == "tech", blush: style == "calm" || universe == "anime",
	}
}

var torsoPath = map[string]string{
	"broad":  "M5 64 C5 51 17 46.5 32 46.5 C47 46.5 59 51 59 64Z",
	"narrow": "M9 64 C9 52 20 47.5 32 47.5 C44 47.5 55 52 55 64Z",
}

const hoodPath = "M12 64 L12 38 C12 15 22 6 32 6 C42 6 52 15 52 38 L52 64Z"
const hoodRim = "M17.6 42 C13.6 24 21.6 8.6 32 8.6 C42.4 8.6 50.4 24 46.4 42 C47.6 26 41.4 15.4 32 15.4 C22.6 15.4 16.4 26 17.6 42Z"

type w struct{ sb strings.Builder }

func (x *w) f(format string, a ...any) { fmt.Fprintf(&x.sb, format, a...) }

func num(v float64) string {
	return strconv.FormatFloat(float64(int64(v*1000+sign(v)*0.5))/1000, 'f', -1, 64)
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

func (x *w) star(cx, cy, r float64, fill string) {
	k := r * 0.28
	x.f(`<path d="M%s %s L%s %s L%s %s L%s %s L%s %s L%s %s L%s %s L%s %sZ" fill="%s"/>`,
		num(cx), num(cy-r), num(cx+k), num(cy-k), num(cx+r), num(cy), num(cx+k), num(cy+k),
		num(cx), num(cy+r), num(cx-k), num(cy+k), num(cx-r), num(cy), num(cx-k), num(cy-k), fill)
}

func cleanPrefix(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// RenderSVG returns a self-contained circular 64-unit bust portrait as inline SVG markup.
// All ids inside are prefixed with idPrefix so several avatars can share one page.
func RenderSVG(universe, gender, style string, size int, idPrefix string) string {
	s := newSpec(universe, gender, style)
	if size <= 0 {
		size = 48
	}
	clip := cleanPrefix(idPrefix) + "avatar-clip"
	p := s.p
	x := &w{}
	x.f(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="%d" height="%d" role="img" aria-hidden="true" focusable="false" style="display:block;flex:none;max-width:100%%">`, size, size)
	x.f(`<defs><clipPath id="%s"><circle cx="32" cy="32" r="32"/></clipPath></defs>`, clip)
	x.f(`<g clip-path="url(#%s)">`, clip)
	s.backdropSVG(x)
	s.backMotif(x)
	s.backHair(x)
	s.torsoSVG(x)
	x.f(`<path d="M28 36 V46.5 Q32 52 36 46.5 V36Z" fill="%s"/>`, s.b.shade)
	s.clothingDetail(x)
	s.earsSVG(x)
	if s.gender == "male" {
		x.f(`<path d="M20 28 C20 17 26 14.5 32 14.5 C38 14.5 44 17 44 28 C44 37 39 43.5 32 43.5 C25 43.5 20 37 20 28Z" fill="%s"/>`, s.b.skin)
	} else {
		x.f(`<path d="M20.6 28 C20.6 17 26 14.5 32 14.5 C38 14.5 43.4 17 43.4 28 C43.4 37 38.6 43.5 32 43.5 C25.4 43.5 20.6 37 20.6 28Z" fill="%s"/>`, s.b.skin)
	}
	s.face(x)
	s.frontHair(x)
	if s.universe == "sports" {
		fill := p.jersey
		if s.style == "tech" {
			fill = p.accent
		}
		x.f(`<path d="M20 24.4 Q32 20.2 44 24.4 L44 27.2 Q32 23 20 27.2Z" fill="%s" stroke="%s" stroke-width="0.6"/>`, fill, p.jersey2)
	}
	if s.visor {
		x.f(`<g><rect x="21" y="26.2" width="22" height="8" rx="3.4" fill="%s" fill-opacity="0.22" stroke="%s" stroke-width="1"/><path d="M23 28.4 H27" stroke="#FFFFFF" stroke-width="0.8" stroke-linecap="round" opacity="0.7"/></g>`, p.accent, p.accent)
	}
	s.frontMotif(x)
	x.f(`</g></svg>`)
	return x.sb.String()
}

func (s spec) backdropSVG(x *w) {
	p := s.p
	x.f(`<rect width="64" height="64" fill="%s"/>`, p.bg)
	switch s.backdrop {
	case "burst":
		x.f(`<path d="M0 44 L44 0 H64 V10 L10 64 H0Z" fill="%s" opacity="0.55"/>`, p.bg2)
		x.f(`<circle cx="54" cy="12" r="2.2" fill="%s"/><circle cx="9" cy="30" r="1.8" fill="%s"/><circle cx="56" cy="40" r="1.6" fill="%s"/>`, p.accent, p.accent, p.cloth)
	case "panel":
		x.f(`<circle cx="32" cy="34" r="25" fill="%s" opacity="0.7"/><rect x="0" y="0" width="5" height="64" fill="%s" opacity="0.6"/>`, p.bg2, p.bg2)
	case "blobs":
		x.f(`<circle cx="14" cy="16" r="15" fill="%s" opacity="0.7"/><circle cx="54" cy="44" r="17" fill="%s" opacity="0.55"/><circle cx="50" cy="10" r="5" fill="%s" opacity="0.8"/>`, p.bg2, p.bg2, p.cloth2)
	default:
		x.f(`<circle cx="32" cy="30" r="26" fill="%s" opacity="0.8"/>`, p.bg2)
		x.f(`<g stroke="%s" stroke-width="1" fill="none" stroke-linecap="round" stroke-linejoin="round" opacity="0.9"><path d="M0 14 H9 L13 10 H21"/><path d="M64 22 H55 L51 18 H44"/><path d="M0 50 H8 L12 54"/><path d="M64 8 H58"/></g>`, p.accent)
		x.f(`<g fill="%s"><circle cx="21" cy="10" r="1.4"/><circle cx="44" cy="18" r="1.4"/><circle cx="12" cy="54" r="1.4"/></g>`, p.accent)
	}
}

func (s spec) backMotif(x *w) {
	p := s.p
	switch s.motif {
	case "clapper":
		x.f(`<g><circle cx="13" cy="13" r="7" fill="%s"/><g fill="%s"><circle cx="13" cy="9" r="1.6"/><circle cx="13" cy="17" r="1.6"/><circle cx="9" cy="13" r="1.6"/><circle cx="17" cy="13" r="1.6"/></g><circle cx="13" cy="13" r="1.3" fill="%s"/></g>`, p.ink, p.bg, p.bg)
	case "sparkles":
		x.star(12, 14, 6, p.accent)
		x.star(53, 11, 4, p.cloth2)
		x.star(55, 30, 2.6, p.accent)
	case "paw":
		x.f(`<g fill="%s" opacity="0.9"><ellipse cx="13" cy="15" rx="3.8" ry="3.2"/><circle cx="8" cy="10.6" r="1.7"/><circle cx="12" cy="8.4" r="1.7"/><circle cx="16.6" cy="9" r="1.7"/></g>`, p.accent)
	case "ball":
		x.f(`<g stroke="%s" stroke-width="1.4" stroke-linecap="round" opacity="0.9"><path d="M5 12 H15"/><path d="M8 17 H18"/><path d="M4 22 H11"/></g>`, p.accent)
	default:
		x.f(`<path d="M51 6 a7 7 0 1 0 6 10 a5.4 5.4 0 1 1 -6 -10Z" fill="%s"/>`, p.metal)
		x.star(12, 13, 4.6, p.metal)
		x.star(55, 30, 2.4, p.cloth2)
	}
}

func (s spec) backHair(x *w) {
	c := s.hairColor
	switch s.hairShape {
	case "wave":
		x.f(`<path d="M17.5 30 C14 12 26 7.5 32 7.5 C38 7.5 50 12 46.5 30 C48.5 38 50 46 45.5 52 C42 49 41 46 41 44 L23 44 C23 46 22 49 18.5 52 C14 46 15.5 38 17.5 30Z" fill="%s"/>`, c)
	case "anime_long":
		x.f(`<path d="M17 30 C14 12 26 8 32 8 C38 8 50 12 47 30 C48 40 49 50 46 56 L18 56 C15 50 16 40 17 30Z" fill="%s"/>`, c)
	case "long_crown":
		x.f(`<path d="M17 30 C14 12 26 8 32 8 C38 8 50 12 47 30 C48.5 41 50 52 46 60 L18 60 C14 52 15.5 41 17 30Z" fill="%s"/>`, c)
	case "ponytail":
		x.f(`<path d="M42 22 C52 22 56 32 52 42 C50 46 47 46 45.5 44 C49 38 47 32 42 30Z" fill="%s"/>`, c)
	case "hood":
		x.f(`<path d="%s" fill="%s"/><path d="%s" fill="%s" opacity="0.25"/>`, hoodPath, s.p.cloth, hoodPath, s.p.bg2)
	}
}

func (s spec) torsoSVG(x *w) {
	p := s.p
	d := torsoPath[s.torso]
	switch s.clothing {
	case "jersey":
		x.f(`<g><path d="%s" fill="%s"/><path d="M5.5 56 C6 52 9 50 12 49 L14.5 64 H5Z" fill="%s" opacity="0.9"/><path d="M58.5 56 C58 52 55 50 52 49 L49.5 64 H59Z" fill="%s" opacity="0.9"/><path d="M28 54.5 H36 L31 63" stroke="%s" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"/></g>`, d, p.jersey, p.jersey2, p.jersey2, p.jersey2)
	case "robe":
		x.f(`<g><path d="%s" fill="%s"/><path d="M32 47 V64" stroke="%s" stroke-width="1.2" opacity="0.7"/><path d="M22 50 L32 58 L42 50" stroke="%s" stroke-width="1.4" fill="none" stroke-linecap="round" stroke-linejoin="round"/></g>`, d, p.cloth, p.cloth2, p.metal)
	default:
		x.f(`<path d="%s" fill="%s"/>`, d, p.cloth)
	}
}

func (s spec) clothingDetail(x *w) {
	p := s.p
	if s.clothing != "style" {
		if s.clothing == "robe" {
			x.f(`<circle cx="32" cy="55" r="2.6" fill="%s"/><circle cx="31.2" cy="54.2" r="0.8" fill="#FFFFFF" opacity="0.9"/>`, p.metal)
			return
		}
		x.f(`<path d="M26.5 46.6 Q32 52.5 37.5 46.6" stroke="%s" stroke-width="2" fill="none" stroke-linecap="round"/>`, p.jersey2)
		return
	}
	switch s.style {
	case "formal":
		x.f(`<path d="M25.5 46.4 L32 59 L38.5 46.4 L36 46.2 L32 52 L28 46.2Z" fill="%s"/><path d="M28 46.4 Q32 52.2 36 46.4 L32 53Z" fill="%s"/>`, p.cloth2, s.b.shade)
		if s.gender == "male" {
			x.f(`<path d="M30.4 52.6 H33.6 L34.2 55.2 H29.8Z" fill="%s"/><path d="M29.8 55.2 H34.2 L35.4 64 L32 66 L28.6 64Z" fill="%s"/>`, p.accent, p.accent)
		} else {
			x.f(`<circle cx="32" cy="56.4" r="1.7" fill="%s"/><path d="M25.5 46.4 L22 52 L27.5 52Z M38.5 46.4 L42 52 L36.5 52Z" fill="%s" opacity="0.55"/>`, p.accent, p.cloth2)
		}
	case "calm":
		x.f(`<path d="M26 46.6 Q32 54 38 46.6" stroke="%s" stroke-width="2.6" fill="none" stroke-linecap="round"/>`, p.cloth2)
	case "tech":
		x.f(`<path d="M26 46.4 L32 57.5 L38 46.4" stroke="%s" stroke-width="1.4" fill="none" stroke-linecap="round" stroke-linejoin="round"/><circle cx="21" cy="58" r="1.3" fill="%s"/><path d="M43 56 H49 V60" stroke="%s" stroke-width="1" fill="none" stroke-linecap="round" stroke-linejoin="round"/>`, p.accent, p.accent, p.accent)
	default:
		x.f(`<path d="M12 58.5 H52" stroke="%s" stroke-width="3" stroke-linecap="round" opacity="0.95"/>`, p.accent)
	}
}

func (s spec) earsSVG(x *w) {
	b := s.b
	switch s.ears {
	case "animal":
		if s.gender == "male" {
			x.f(`<path d="M18.5 25 L19.5 6 L32 16Z" fill="%s"/><path d="M45.5 25 L44.5 6 L32 16Z" fill="%s"/><path d="M21.5 20 L21.8 11.5 L27.5 16Z" fill="%s"/><path d="M42.5 20 L42.2 11.5 L36.5 16Z" fill="%s"/>`, b.skin, b.skin, b.inner, b.inner)
		} else {
			x.f(`<path d="M18 26 C16.5 16 18.5 9 21 8 C24 9 29 13 31 17Z" fill="%s"/><path d="M46 26 C47.5 16 45.5 9 43 8 C40 9 35 13 33 17Z" fill="%s"/><path d="M20 21 C19.5 15 20.5 12 21.4 11.4 C23 12.4 26 15 27 17.5Z" fill="%s"/><path d="M44 21 C44.5 15 43.5 12 42.6 11.4 C41 12.4 38 15 37 17.5Z" fill="%s"/>`, b.skin, b.skin, b.inner, b.inner)
		}
	case "elf":
		x.f(`<path d="M20.6 32 L12.5 22.5 L21 27Z" fill="%s"/><path d="M43.4 32 L51.5 22.5 L43 27Z" fill="%s"/>`, b.skin, b.skin)
	default:
		x.f(`<circle cx="20.4" cy="30.5" r="2.7" fill="%s"/><circle cx="43.6" cy="30.5" r="2.7" fill="%s"/>`, b.shade, b.shade)
		if s.style == "tech" {
			x.f(`<circle cx="20.4" cy="30.5" r="1.1" fill="%s"/>`, s.p.accent)
		}
	}
}

func (s spec) mouthSVG(x *w) {
	ink := s.p.ink
	if s.universe == "anime" {
		x.f(`<path d="M29.6 37 Q32 39.4 34.4 37" stroke="%s" stroke-width="1.1" fill="none" stroke-linecap="round"/>`, ink)
		return
	}
	switch s.mouth {
	case "grin":
		x.f(`<path d="M27.8 36.2 Q32 41.8 36.2 36.2 Q32 37.8 27.8 36.2Z" fill="#8E1F2E" stroke="%s" stroke-width="0.9" stroke-linejoin="round"/>`, ink)
	case "slight":
		x.f(`<path d="M29.2 37.4 Q32 38.9 34.8 37.4" stroke="%s" stroke-width="1.2" fill="none" stroke-linecap="round"/>`, ink)
	case "soft":
		x.f(`<path d="M29 36.8 Q32 39.8 35 36.8" stroke="%s" stroke-width="1.2" fill="none" stroke-linecap="round"/>`, ink)
	default:
		x.f(`<path d="M29.4 37.6 H34.6" stroke="%s" stroke-width="1.3" fill="none" stroke-linecap="round"/>`, ink)
	}
}

func (s spec) face(x *w) {
	p := s.p
	ink := p.ink
	animal := s.universe == "animals"
	eyeY := 30.0
	if animal {
		eyeY = 29.5
	}
	x.f(`<g>`)
	if animal {
		x.f(`<path d="M20.2 31 Q27 31.5 32 40.5 Q37 31.5 43.8 31 Q44 41 32 44 Q20 41 20.2 31Z" fill="%s"/><ellipse cx="32" cy="36" rx="2" ry="1.4" fill="%s"/>`, s.b.light, ink)
		x.f(`<path d="M32 37.2 V38.8 M29.6 39.3 Q32 41.4 34.4 39.3" stroke="%s" stroke-width="1" fill="none" stroke-linecap="round"/>`, ink)
		x.f(`<g stroke="%s" stroke-width="0.7" stroke-linecap="round" opacity="0.7"><path d="M21 36 L14 34.6 M21 38.4 L14.5 39.4"/><path d="M43 36 L50 34.6 M43 38.4 L49.5 39.4"/></g>`, ink)
	}
	if s.anime {
		x.f(`<ellipse cx="27" cy="30.4" rx="2.9" ry="3.7" fill="%s"/><ellipse cx="37" cy="30.4" rx="2.9" ry="3.7" fill="%s"/>`, ink, ink)
		x.f(`<ellipse cx="27" cy="31" rx="2.2" ry="3" fill="%s"/><ellipse cx="37" cy="31" rx="2.2" ry="3" fill="%s"/>`, p.eye, p.eye)
		x.f(`<circle cx="26.1" cy="29.2" r="1.15" fill="#FFFFFF"/><circle cx="36.1" cy="29.2" r="1.15" fill="#FFFFFF"/><circle cx="28" cy="32.4" r="0.5" fill="#FFFFFF"/><circle cx="38" cy="32.4" r="0.5" fill="#FFFFFF"/>`)
	} else {
		x.f(`<ellipse cx="27.4" cy="%s" rx="1.5" ry="1.8" fill="%s"/><ellipse cx="36.6" cy="%s" rx="1.5" ry="1.8" fill="%s"/>`, num(eyeY), ink, num(eyeY), ink)
		x.f(`<circle cx="27.9" cy="%s" r="0.5" fill="#FFFFFF"/><circle cx="37.1" cy="%s" r="0.5" fill="#FFFFFF"/>`, num(eyeY-0.6), num(eyeY-0.6))
	}
	if !animal && s.universe != "anime" && s.hairShape != "hood" {
		sw := "1.1"
		if s.style == "formal" {
			sw = "1.5"
		}
		x.f(`<g stroke="%s" stroke-width="%s" stroke-linecap="round" fill="none"><path d="M24.6 26 Q27.2 24.6 29.8 25.8"/><path d="M34.2 25.8 Q36.8 24.6 39.4 26"/></g>`, s.hairColor, sw)
	}
	if s.blush {
		x.f(`<g fill="%s" opacity="0.7"><ellipse cx="24.2" cy="35.2" rx="2.4" ry="1.4"/><ellipse cx="39.8" cy="35.2" rx="2.4" ry="1.4"/></g>`, p.blush)
	}
	if !animal {
		s.mouthSVG(x)
	}
	if !animal && s.universe != "anime" {
		x.f(`<path d="M32 31.5 Q31 34.4 32.6 34.9" stroke="%s" stroke-width="1" fill="none" stroke-linecap="round"/>`, s.b.shade)
	}
	x.f(`</g>`)
}

func (s spec) frontHair(x *w) {
	c := s.hairColor
	p := s.p
	switch s.hairShape {
	case "swept":
		x.f(`<path d="M19.8 29 C18.6 15 26 11 32 11 C39 11 45.6 15.5 44.2 29 C43 23 39.5 19.6 33 19.4 C27 19.2 22 22 19.8 29Z" fill="%s"/>`, c)
	case "short":
		x.f(`<path d="M19.8 28 C19 16 25.5 12 32 12 C38.5 12 45 16 44.2 28 C42.6 22.4 38.6 20.4 32 20.4 C25.4 20.4 21.4 22.4 19.8 28Z" fill="%s"/>`, c)
	case "wave":
		x.f(`<path d="M20 29 C18 15 25 11 32 11 C39 11 46 15 44 29 C43 24 40 19 34 17.6 C30 21 25 22.6 20 29Z" fill="%s"/>`, c)
	case "ponytail":
		x.f(`<path d="M20.2 27.5 C19.4 16 25.5 12.5 32 12.5 C38.5 12.5 44.6 16 43.8 27.5 C42 22.2 38 20.2 32 20.2 C26 20.2 22 22.2 20.2 27.5Z" fill="%s"/>`, c)
	case "spiky":
		x.f(`<path d="M19.4 29 C17.6 21 19.4 15 23.4 12.6 L22.8 6.8 L28.4 11.2 L32 4.6 L35.6 11.2 L41.2 6.8 L40.6 12.6 C44.6 15 46.4 21 44.6 29 C43 24 40.4 20.4 38 19 L34 22.6 L30 19 C26.4 20.4 22 24 19.4 29Z" fill="%s"/>`, c)
	case "anime_long":
		x.f(`<path d="M19.6 30 C17.6 16 25.4 10.6 32 10.6 C38.6 10.6 46.4 16 44.4 30 C43.6 24 41 20.4 37 19 C35 22 30 22 27.4 19 C23.4 20.4 20.6 24 19.6 30Z" fill="%s"/>`, c)
		x.f(`<path d="M19.6 30 C17.4 38 17.6 48 20.4 54 L23.8 54 C22 47 22.6 38 23.6 31Z" fill="%s"/><path d="M44.4 30 C46.6 38 46.4 48 43.6 54 L40.2 54 C42 47 41.4 38 40.4 31Z" fill="%s"/>`, c, c)
		bow := p.accent
		if s.style == "tech" {
			bow = p.cloth2
		}
		x.f(`<circle cx="41.4" cy="17" r="2.1" fill="%s"/><circle cx="41.4" cy="17" r="0.8" fill="#FFFFFF"/>`, bow)
	case "long_crown":
		x.f(`<path d="M19.8 29.5 C18.6 17 25.6 12.6 32 12.6 C38.4 12.6 45.4 17 44.2 29.5 C42.6 24 38.6 19.6 32.6 19.4 C26 19.2 21.8 23.6 19.8 29.5Z" fill="%s"/>`, c)
		x.f(`<path d="M22.2 21 L23.8 14.4 L27.6 18.6 L32 12 L36.4 18.6 L40.2 14.4 L41.8 21Z" fill="%s" stroke="%s" stroke-width="0.5" stroke-linejoin="round"/><circle cx="32" cy="17.6" r="1.1" fill="#FFFFFF" opacity="0.9"/>`, p.metal, p.ink)
	case "hood":
		x.f(`<path d="%s" fill="%s"/><path d="%s" fill="%s" opacity="0.25"/>`, hoodRim, p.cloth, hoodRim, p.bg2)
	}
}

func (s spec) frontMotif(x *w) {
	ink := s.p.ink
	switch s.motif {
	case "clapper":
		x.f(`<g><rect x="37.5" y="45.5" width="16" height="9.2" rx="1" fill="%s"/><path d="M40 48.4 H51 M40 51.6 H48" stroke="#FFFFFF" stroke-width="1" stroke-linecap="round" opacity="0.85"/><g transform="rotate(-14 37.5 45.5)"><rect x="37.5" y="41.6" width="16" height="3.8" rx="0.8" fill="%s"/><path d="M41 41.6 L43.4 45.4 H45.4 L43 41.6Z M46.4 41.6 L48.8 45.4 H50.8 L48.4 41.6Z" fill="#FFFFFF"/></g></g>`, ink, ink)
	case "ball":
		x.f(`<g><circle cx="17.5" cy="49.5" r="6" fill="#FFFFFF" stroke="%s" stroke-width="0.9"/><path d="M17.5 46.6 L20.2 48.6 L19.2 51.8 H15.8 L14.8 48.6Z" fill="%s"/><path d="M17.5 46.6 V44 M20.2 48.6 L22.8 47.6 M19.2 51.8 L21 54 M15.8 51.8 L14 54 M14.8 48.6 L12.2 47.6" stroke="%s" stroke-width="0.8" stroke-linecap="round"/></g>`, ink, ink, ink)
	}
}
