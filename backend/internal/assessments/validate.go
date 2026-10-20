// Package assessments: creation and validation of assessment definitions.
// Validation is pure (no DB): callers pass the set of known characteristic ids.
package assessments

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/scoring"
)

// Limits mirror the CHECK constraints in 001_schema.sql.
const (
	maxTitle       = 200
	maxDescription = 4000
	maxQuestion    = 1000
	maxOption      = 500
	minOptions     = 2
	maxRatingOpts  = 10
)

// Question types.
const (
	TypeSingle   = "single"
	TypeMultiple = "multiple"
	TypeRating   = "rating"
)

// levelWeight maps the qualitative level to its weight (plan section 5).
var levelWeight = map[string]int{"low": 1, "moderate": 2, "high": 3}

// Warning and validation error codes.
const (
	WarnMinUnreachable = "min_unreachable"

	CodeRequired                = "required"
	CodeTooLong                 = "too_long"
	CodePastExpiry              = "past_expiry"
	CodeInvalidType             = "invalid_type"
	CodeTooFewOptions           = "too_few_options"
	CodeTooManyOptions          = "too_many_options"
	CodeUnknownCharacteristic   = "unknown_characteristic"
	CodeDuplicateCharacteristic = "duplicate_characteristic"
	CodeInvalidLevel            = "invalid_level"
	CodeInvalidDirection        = "invalid_direction"
	CodeNoMeasurable            = "no_measurable_characteristic"
)

func runes(s string) int { return utf8.RuneCountInString(strings.TrimSpace(s)) }

// Validate checks a creation request. known is the set of existing characteristic ids.
// It returns blocking errors (422) and, only when there are none, non-blocking warnings.
func Validate(req dto.CreateAssessmentRequest, known map[int64]bool, now time.Time) ([]dto.FieldError, []dto.Warning) {
	var errs []dto.FieldError
	add := func(path, code string) { errs = append(errs, dto.FieldError{Path: path, Code: code}) }

	switch n := runes(req.Title); {
	case n == 0:
		add("title", CodeRequired)
	case n > maxTitle:
		add("title", CodeTooLong)
	}
	if utf8.RuneCountInString(req.Description) > maxDescription {
		add("description", CodeTooLong)
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(now) {
		add("expiresAt", CodePastExpiry)
	}
	if len(req.Questions) == 0 {
		add("questions", CodeRequired)
	}

	for qi, q := range req.Questions {
		qp := fmt.Sprintf("questions[%d]", qi)
		switch n := runes(q.Text); {
		case n == 0:
			add(qp+".text", CodeRequired)
		case n > maxQuestion:
			add(qp+".text", CodeTooLong)
		}
		switch q.Type {
		case TypeSingle, TypeMultiple, TypeRating:
		default:
			add(qp+".type", CodeInvalidType)
		}
		if len(q.Options) < minOptions {
			add(qp+".options", CodeTooFewOptions)
		}
		if q.Type == TypeRating && len(q.Options) > maxRatingOpts {
			add(qp+".options", CodeTooManyOptions)
		}
		for oi, o := range q.Options {
			op := fmt.Sprintf("%s.options[%d]", qp, oi)
			switch n := runes(o.Text); {
			case n == 0:
				add(op+".text", CodeRequired)
			case n > maxOption:
				add(op+".text", CodeTooLong)
			}
			seen := map[int64]bool{}
			for ei, e := range o.Effects {
				ep := fmt.Sprintf("%s.effects[%d]", op, ei)
				if !known[e.CharacteristicID] {
					add(ep+".characteristicId", CodeUnknownCharacteristic)
				} else if seen[e.CharacteristicID] {
					add(ep+".characteristicId", CodeDuplicateCharacteristic)
				}
				seen[e.CharacteristicID] = true
				if _, ok := levelWeight[e.Level]; !ok {
					add(ep+".level", CodeInvalidLevel)
				}
				if e.Direction != 1 && e.Direction != -1 {
					add(ep+".direction", CodeInvalidDirection)
				}
			}
		}
	}
	if len(errs) > 0 {
		return errs, nil
	}

	// MAX(c) per characteristic; c is measurable when MAX(c) > 0 (plan section 5).
	qs := toScoring(req.Questions)
	measurable := false
	for _, m := range scoring.MaxTotals(qs) {
		if m > 0 {
			measurable = true
			break
		}
	}
	if !measurable {
		return []dto.FieldError{{Path: "questions", Code: CodeNoMeasurable}}, nil
	}

	// Amendment 3: assessment-wide minimum reachable total for c; > 0 means 0% is unreachable.
	// scoring.MinUnreachable is the single source of truth (also used by the read side).
	var warns []dto.Warning
	for _, c := range scoring.MinUnreachable(qs) {
		warns = append(warns, dto.Warning{Path: "questions", Code: WarnMinUnreachable, CharacteristicID: c})
	}
	return nil, warns
}

// toScoring maps a validated request onto the scoring model (signed effects per option).
func toScoring(in []dto.CreateQuestionRequest) []scoring.Question {
	qs := make([]scoring.Question, 0, len(in))
	for _, q := range in {
		sq := scoring.Question{Type: q.Type, Options: make([]scoring.Option, 0, len(q.Options))}
		for _, o := range q.Options {
			eff := map[int64]int{}
			for _, e := range o.Effects {
				eff[e.CharacteristicID] = scoring.Signed(e.Direction, e.Level)
			}
			sq.Options = append(sq.Options, scoring.Option{Effects: eff})
		}
		qs = append(qs, sq)
	}
	return qs
}
