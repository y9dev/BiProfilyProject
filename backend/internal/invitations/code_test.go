package invitations

import (
	"regexp"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	re := regexp.MustCompile(`^[A-Z2-7]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		c, err := GenerateCode()
		if err != nil || !re.MatchString(c) {
			t.Fatalf("bad code %q (%v)", c, err)
		}
		if seen[c] {
			t.Fatalf("duplicate code %q", c)
		}
		seen[c] = true
	}
}

func TestNormalizeCode(t *testing.T) {
	if got := NormalizeCode("  abc234 "); got != "ABC234" {
		t.Fatalf("got %q", got)
	}
}
