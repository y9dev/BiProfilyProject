package invitations

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

// CodeLen is the invitation code length: 12 base32 chars = 60 bits of entropy.
const CodeLen = 12

var codeEnc = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateCode returns a fresh crypto/rand code (A-Z, 2-7). The caller retries
// on a UNIQUE collision.
func GenerateCode() (string, error) {
	b := make([]byte, 8) // 64 bits -> 13 chars, first 12 kept
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return codeEnc.EncodeToString(b)[:CodeLen], nil
}

// NormalizeCode trims and upper-cases user input before lookup.
func NormalizeCode(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }
