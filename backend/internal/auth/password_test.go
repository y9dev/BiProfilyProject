package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct-horse-8-symbols")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword("correct-horse-8-symbols", h)
	if err != nil || !ok {
		t.Fatalf("verify own password: ok=%v err=%v", ok, err)
	}
	ok, _ = VerifyPassword("wrong-password", h)
	if ok {
		t.Fatal("wrong password accepted")
	}
}

func TestPasswordSaltsDiffer(t *testing.T) {
	a, _ := HashPassword("same-password-123")
	b, _ := HashPassword("same-password-123")
	if a == b {
		t.Fatal("two hashes of one password are identical — no salt?")
	}
}
