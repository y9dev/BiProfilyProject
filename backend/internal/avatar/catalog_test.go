package avatar

import (
	"testing"

	"bigroup-backend/internal/model"
)

func TestValid(t *testing.T) {
	cases := []struct {
		a  model.Avatar
		ok bool
	}{
		{model.Avatar{Universe: "anime", Gender: "male", Style: "tech"}, true},
		{model.Avatar{Universe: "movies", Gender: "female", Style: "calm"}, true},
		{model.Avatar{Universe: "nope", Gender: "male", Style: "tech"}, false},
		{model.Avatar{Universe: "anime", Gender: "x", Style: "tech"}, false},
		{model.Avatar{Universe: "anime", Gender: "male", Style: ""}, false},
		{model.Avatar{}, false},
	}
	for _, c := range cases {
		if got := Valid(c.a); got != c.ok {
			t.Errorf("Valid(%+v) = %v, want %v", c.a, got, c.ok)
		}
	}
}
