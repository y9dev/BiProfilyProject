package scoring

import (
	"slices"
	"testing"
)

// o builds an option with signed effects {charID: signed}.
func o(id int64, eff map[int64]int) Option { return Option{ID: id, Effects: eff} }

func scoreOf(rs []Result, c int64) (Result, bool) {
	for _, r := range rs {
		if r.CharacteristicID == c {
			return r, true
		}
	}
	return Result{}, false
}

func TestSigned(t *testing.T) {
	cases := []struct {
		dir   int
		level string
		want  int
	}{{1, "low", 1}, {1, "moderate", 2}, {1, "high", 3}, {-1, "high", -3}, {-1, "low", -1}, {1, "bogus", 0}}
	for _, c := range cases {
		if got := Signed(c.dir, c.level); got != c.want {
			t.Errorf("Signed(%d,%s)=%d want %d", c.dir, c.level, got, c.want)
		}
	}
}

func TestCompute(t *testing.T) {
	cases := []struct {
		name    string
		qs      []Question
		answers map[int64][]int64
		want    map[int64]Result
		absent  []int64
	}{
		{
			name:    "single best",
			qs:      []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(10, map[int64]int{1: 3}), o(11, map[int64]int{1: 1}), o(12, nil)}}},
			answers: map[int64][]int64{1: {10}},
			want:    map[int64]Result{1: {CharacteristicID: 1, Raw: 3, Max: 3, Score: 100}},
		},
		{
			name:    "single mid rounds 66.67",
			qs:      []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(10, map[int64]int{1: 3}), o(11, map[int64]int{1: 2})}}},
			answers: map[int64][]int64{1: {11}},
			want:    map[int64]Result{1: {CharacteristicID: 1, Raw: 2, Max: 3, Score: 66.67}},
		},
		{
			name:    "rating behaves as single",
			qs:      []Question{{ID: 1, Type: TypeRating, Options: []Option{o(10, map[int64]int{1: -2}), o(11, nil), o(12, map[int64]int{1: 2})}}},
			answers: map[int64][]int64{1: {12}},
			want:    map[int64]Result{1: {CharacteristicID: 1, Raw: 2, Max: 2, Score: 100}},
		},
		{
			name:    "multiple sums positives for max and chosen for raw",
			qs:      []Question{{ID: 1, Type: TypeMultiple, Options: []Option{o(10, map[int64]int{1: 3}), o(11, map[int64]int{1: 1}), o(12, map[int64]int{1: -2})}}},
			answers: map[int64][]int64{1: {10, 12}},
			want:    map[int64]Result{1: {CharacteristicID: 1, Raw: 1, Max: 4, Score: 25}},
		},
		{
			name:    "negative total clamped to 0",
			qs:      []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(10, map[int64]int{1: 2}), o(11, map[int64]int{1: -3})}}},
			answers: map[int64][]int64{1: {11}},
			want:    map[int64]Result{1: {CharacteristicID: 1, Raw: -3, Max: 2, Score: 0}},
		},
		{
			name: "clamp is per characteristic after summing, not per question",
			qs: []Question{
				{ID: 1, Type: TypeSingle, Options: []Option{o(10, map[int64]int{1: 3}), o(11, map[int64]int{1: -3})}},
				{ID: 2, Type: TypeSingle, Options: []Option{o(20, map[int64]int{1: 3}), o(21, map[int64]int{1: -3})}},
			},
			answers: map[int64][]int64{1: {10}, 2: {21}}, // +3 + -3 = 0 (per-question clamp would give 3)
			want:    map[int64]Result{1: {CharacteristicID: 1, Raw: 0, Max: 6, Score: 0}},
		},
		{
			name:    "MAX zero is unmeasured: no row",
			qs:      []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(10, map[int64]int{1: -1, 2: 2}), o(11, map[int64]int{1: -2})}}},
			answers: map[int64][]int64{1: {10}},
			want:    map[int64]Result{2: {CharacteristicID: 2, Raw: 2, Max: 2, Score: 100}},
			absent:  []int64{1},
		},
		{
			name: "two characteristics independent",
			qs: []Question{
				{ID: 1, Type: TypeSingle, Options: []Option{o(10, map[int64]int{1: 3, 2: 1}), o(11, map[int64]int{1: 1, 2: 3})}},
			},
			answers: map[int64][]int64{1: {11}},
			want: map[int64]Result{
				1: {CharacteristicID: 1, Raw: 1, Max: 3, Score: 33.33},
				2: {CharacteristicID: 2, Raw: 3, Max: 3, Score: 100},
			},
		},
		{
			name: "option without effect on c adds nothing",
			qs: []Question{
				{ID: 1, Type: TypeSingle, Options: []Option{o(10, map[int64]int{1: 2}), o(11, nil)}},
				{ID: 2, Type: TypeMultiple, Options: []Option{o(20, map[int64]int{1: 1}), o(21, nil)}},
			},
			answers: map[int64][]int64{1: {11}, 2: {21}},
			want:    map[int64]Result{1: {CharacteristicID: 1, Raw: 0, Max: 3, Score: 0}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compute(tc.qs, tc.answers)
			for c, want := range tc.want {
				r, ok := scoreOf(got, c)
				if !ok || r != want {
					t.Errorf("c%d = %+v (ok=%v), want %+v", c, r, ok, want)
				}
			}
			for _, c := range tc.absent {
				if _, ok := scoreOf(got, c); ok {
					t.Errorf("c%d must have no score row", c)
				}
			}
			if len(got) != len(tc.want) {
				t.Errorf("got %d rows, want %d: %+v", len(got), len(tc.want), got)
			}
		})
	}
}

func TestComputeOrderedByCharacteristic(t *testing.T) {
	qs := []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(10, map[int64]int{5: 1, 2: 1, 9: 1})}}}
	got := Compute(qs, map[int64][]int64{1: {10}})
	ids := []int64{}
	for _, r := range got {
		ids = append(ids, r.CharacteristicID)
	}
	if !slices.Equal(ids, []int64{2, 5, 9}) {
		t.Fatalf("order %v", ids)
	}
}

func TestNormalizeRounding(t *testing.T) {
	cases := []struct {
		raw, max int
		want     float64
	}{{1, 3, 33.33}, {2, 3, 66.67}, {1, 6, 16.67}, {5, 5, 100}, {9, 5, 100}, {-4, 5, 0}, {0, 0, 0}}
	for _, c := range cases {
		if got := Normalize(c.raw, c.max); got != c.want {
			t.Errorf("Normalize(%d,%d)=%v want %v", c.raw, c.max, got, c.want)
		}
	}
}

func TestMean(t *testing.T) {
	if got := Mean([]float64{66.67, 78.33}); got != 72.5 {
		t.Errorf("mean %v", got)
	}
	if got := Mean(nil); got != 0 {
		t.Errorf("mean(nil) %v", got)
	}
	if got := Mean([]float64{100, 0, 0}); got != 33.33 {
		t.Errorf("mean %v", got)
	}
}

func TestMinUnreachable(t *testing.T) {
	cases := []struct {
		name string
		qs   []Question
		want []int64
	}{
		{"single: every option positive", []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(1, map[int64]int{1: 1}), o(2, map[int64]int{1: 3})}}}, []int64{1}},
		{"single: one option has no effect", []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(1, map[int64]int{1: 1}), o(2, nil)}}}, nil},
		{"single: negative option", []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(1, map[int64]int{1: 2}), o(2, map[int64]int{1: -1})}}}, nil},
		{"multiple: all options positive (non-empty required)", []Question{{ID: 1, Type: TypeMultiple, Options: []Option{o(1, map[int64]int{1: 2}), o(2, map[int64]int{1: 1})}}}, []int64{1}},
		{"multiple: one neutral option", []Question{{ID: 1, Type: TypeMultiple, Options: []Option{o(1, map[int64]int{1: 2}), o(2, nil)}}}, nil},
		{"multiple: negative option cancels", []Question{{ID: 1, Type: TypeMultiple, Options: []Option{o(1, map[int64]int{1: 2}), o(2, map[int64]int{1: -1})}}}, nil},
		{"rating: all positive", []Question{{ID: 1, Type: TypeRating, Options: []Option{o(1, map[int64]int{1: 1}), o(2, map[int64]int{1: 2}), o(3, map[int64]int{1: 3})}}}, []int64{1}},
		{
			"assessment-wide: forced positive offset by forced negative elsewhere",
			[]Question{
				{ID: 1, Type: TypeSingle, Options: []Option{o(1, map[int64]int{1: 1}), o(2, map[int64]int{1: 2})}},   // min +1
				{ID: 2, Type: TypeSingle, Options: []Option{o(3, map[int64]int{1: -3}), o(4, map[int64]int{1: -1})}}, // min -3
			},
			nil,
		},
		{
			"assessment-wide: one forced question, other has neutral option",
			[]Question{
				{ID: 1, Type: TypeSingle, Options: []Option{o(1, map[int64]int{1: 1}), o(2, map[int64]int{1: 2})}},
				{ID: 2, Type: TypeSingle, Options: []Option{o(3, map[int64]int{1: 3}), o(4, nil)}},
			},
			[]int64{1},
		},
		{"multiple characteristics: only the forced one", []Question{{ID: 1, Type: TypeSingle, Options: []Option{o(1, map[int64]int{1: 1, 2: 1}), o(2, map[int64]int{1: 2})}}}, []int64{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MinUnreachable(tc.qs)
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMaxTotals(t *testing.T) {
	qs := []Question{
		{ID: 1, Type: TypeMultiple, Options: []Option{o(1, map[int64]int{1: 3}), o(2, map[int64]int{1: 2, 2: -1}), o(3, map[int64]int{1: -2})}},
		{ID: 2, Type: TypeSingle, Options: []Option{o(4, map[int64]int{1: 1}), o(5, map[int64]int{1: 3})}},
	}
	got := MaxTotals(qs)
	if got[1] != 8 || got[2] != 0 {
		t.Fatalf("%v", got)
	}
}
