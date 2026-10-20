// Package avatar is the server-side avatar catalog: the allowed
// (universe, gender, style) ids. Display names live in the frontend i18n.
package avatar

import (
	"slices"

	"bigroup-backend/internal/model"
)

var (
	Universes = []string{"movies", "anime", "animals", "sports", "fantasy"}
	Genders   = []string{"male", "female"}
	Styles    = []string{"vibrant", "formal", "calm", "tech"}
)

// Valid reports whether the triple is entirely inside the catalog.
func Valid(a model.Avatar) bool {
	return slices.Contains(Universes, a.Universe) &&
		slices.Contains(Genders, a.Gender) &&
		slices.Contains(Styles, a.Style)
}
