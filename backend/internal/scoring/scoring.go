// Package scoring is the pure scoring formula (plan section 5): no DB, no I/O.
// score(c) = clamp(RAW(c), 0, MAX(c)) / MAX(c) * 100, rounded to 2 decimals.
package scoring

import (
	"math"
	"slices"
)

// Question types.
const (
	TypeSingle   = "single"
	TypeMultiple = "multiple"
	TypeRating   = "rating"
)

// Weight maps a qualitative level to its magnitude (low 1, moderate 2, high 3); 0 if unknown.
func Weight(level string) int {
	switch level {
	case "low":
		return 1
	case "moderate":
		return 2
	case "high":
		return 3
	}
	return 0
}

// Signed is the signed effect: direction x weight(level).
func Signed(direction int, level string) int { return direction * Weight(level) }

// Option carries the signed effect of one option per characteristic (absent = 0).
type Option struct {
	ID      int64
	Effects map[int64]int
}

// Question is one question with its options.
type Question struct {
	ID      int64
	Type    string
	Options []Option
}

// Result is a normalized score for a measurable characteristic.
type Result struct {
	CharacteristicID int64
	Raw              int
	Max              int
	Score            float64
}

func isMultiple(q Question) bool { return q.Type == TypeMultiple }

func characteristics(qs []Question) []int64 {
	set := map[int64]bool{}
	for _, q := range qs {
		for _, o := range q.Options {
			for c := range o.Effects {
				set[c] = true
			}
		}
	}
	out := make([]int64, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	slices.Sort(out)
	return out
}

// questionMax is max_q(c).
func questionMax(q Question, c int64) int {
	if isMultiple(q) {
		sum := 0
		for _, o := range q.Options {
			if e := o.Effects[c]; e > 0 {
				sum += e
			}
		}
		return sum
	}
	best := 0
	for _, o := range q.Options {
		if e := o.Effects[c]; e > best {
			best = e
		}
	}
	return best
}

// MaxTotals returns MAX(c) for every characteristic that has an effect somewhere (may be 0).
func MaxTotals(qs []Question) map[int64]int {
	out := map[int64]int{}
	for _, c := range characteristics(qs) {
		for _, q := range qs {
			out[c] += questionMax(q, c)
		}
	}
	return out
}

// Normalize is clamp(raw, 0, max) / max * 100 rounded to 2 decimals; 0 when max <= 0.
func Normalize(raw, max int) float64 {
	if max <= 0 {
		return 0
	}
	r := min(max, raw)
	if r < 0 {
		r = 0
	}
	return round2(float64(r) / float64(max) * 100)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// Compute scores one submission. answers maps question id to the chosen option ids
// (the caller has already validated shape). Characteristics with MAX == 0 produce no
// result. Output is ordered by characteristic id.
func Compute(qs []Question, answers map[int64][]int64) []Result {
	raw := map[int64]int{}
	for _, q := range qs {
		chosen := map[int64]bool{}
		for _, id := range answers[q.ID] {
			chosen[id] = true
		}
		for _, o := range q.Options {
			if !chosen[o.ID] {
				continue
			}
			for c, e := range o.Effects {
				raw[c] += e
			}
		}
	}
	maxes := MaxTotals(qs)
	var out []Result
	for _, c := range characteristics(qs) {
		if maxes[c] == 0 {
			continue
		}
		out = append(out, Result{CharacteristicID: c, Raw: raw[c], Max: maxes[c], Score: Normalize(raw[c], maxes[c])})
	}
	return out
}

// questionMin is the smallest effect on c a valid (non-empty) answer to q can produce.
func questionMin(q Question, c int64) int {
	if len(q.Options) == 0 {
		return 0
	}
	smallest := math.MaxInt
	neg := 0
	for _, o := range q.Options {
		e := o.Effects[c]
		smallest = min(smallest, e)
		if e < 0 {
			neg += e
		}
	}
	if isMultiple(q) && neg < 0 {
		return neg // choose every negative option
	}
	return smallest
}

// MinUnreachable lists characteristics whose 0% is unreachable assessment-wide: the sum
// over questions of each question's minimum possible effect is > 0 (plan sec. 17 #3).
// Ordered by characteristic id. Non-blocking: callers surface it as a warning.
func MinUnreachable(qs []Question) []int64 {
	var out []int64
	for _, c := range characteristics(qs) {
		sum := 0
		for _, q := range qs {
			sum += questionMin(q, c)
		}
		if sum > 0 {
			out = append(out, c)
		}
	}
	return out
}

// Mean averages scores (profile mean over completed assessments), rounded to 2 decimals; 0 for none.
func Mean(scores []float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	return round2(sum / float64(len(scores)))
}
