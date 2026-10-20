// Package auth — пароли, сессии, проверки доступа.
// Криптография: argon2id (рекомендация OWASP для новых систем).
package auth

import (
	"github.com/alexedwards/argon2id"
)

// HashPassword хеширует пароль со случайной солью (внутри библиотеки).
func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

// VerifyPassword сверяет пароль с хешем за константное время.
func VerifyPassword(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}
