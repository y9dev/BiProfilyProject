package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	SessionCookie = "bi_sid"
	CsrfCookie    = "bi_csrf"
	CsrfHeader    = "X-CSRF-Token"

	sessionTTL    = 24 * time.Hour
	sessionMaxAge = 7 * 24 * time.Hour
)

var ErrNoSession = errors.New("no session")

// Session — серверное состояние. Роль всегда читается отсюда,
// никогда из запроса клиента.
type Session struct {
	UserID  int64     `json:"userId"`
	Role    string    `json:"role"`
	Created time.Time `json:"created"`
	// ProfileComplete caches "avatar_set_at IS NOT NULL". When false, the avatar
	// gate re-reads the DB and upgrades the session in place (RequireProfileComplete).
	ProfileComplete bool `json:"profileComplete"`
}

type Manager struct {
	rdb    *redis.Client
	secure bool
}

func NewManager(rdb *redis.Client, cookieSecure bool) *Manager {
	return &Manager{rdb: rdb, secure: cookieSecure}
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func keyOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sess:" + hex.EncodeToString(sum[:])
}

func newCsrfToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewCsrfToken выдаёт токен для double-submit-cookie.
func NewCsrfToken() (string, error) { return newCsrfToken() }

// Create выдаёт новую сессию (ротация: старый токен вызывающая сторона чистит сама).
func (m *Manager) Create(ctx context.Context, userID int64, role string, profileComplete bool) (token string, err error) {
	token, err = newToken()
	if err != nil {
		return "", err
	}
	s := Session{UserID: userID, Role: role, Created: time.Now().UTC(), ProfileComplete: profileComplete}
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	if err := m.rdb.Set(ctx, keyOf(token), raw, sessionTTL).Err(); err != nil {
		return "", err
	}
	_ = m.rdb.SAdd(ctx, userIndex(userID), keyOf(token)).Err()
	_ = m.rdb.Expire(ctx, userIndex(userID), sessionMaxAge).Err()
	return token, nil
}

// Load проверяет токен, продлевает sliding TTL, отсекает по абсолютному сроку.
func (m *Manager) Load(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrNoSession
	}
	raw, err := m.rdb.Get(ctx, keyOf(token)).Bytes()
	if err != nil {
		return nil, ErrNoSession
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, ErrNoSession
	}
	if time.Since(s.Created) > sessionMaxAge {
		_ = m.rdb.Del(ctx, keyOf(token)).Err()
		return nil, ErrNoSession
	}
	_ = m.rdb.Expire(ctx, keyOf(token), sessionTTL).Err()
	return &s, nil
}

// UpdateProfileComplete rewrites the ProfileComplete flag of one session in place:
// same token, remaining TTL kept (Redis SET KEEPTTL). Other sessions of the user
// self-heal through RequireProfileComplete's DB re-read.
func (m *Manager) UpdateProfileComplete(ctx context.Context, token string, complete bool) error {
	if token == "" {
		return ErrNoSession
	}
	return m.setProfileComplete(ctx, keyOf(token), complete)
}

// UpdateProfileCompleteForUser rewrites the flag in every live session of the user
// (PUT /api/me/avatar), so a second browser does not wait for its DB re-read.
func (m *Manager) UpdateProfileCompleteForUser(ctx context.Context, userID int64, complete bool) {
	keys, err := m.rdb.SMembers(ctx, userIndex(userID)).Result()
	if err != nil {
		return
	}
	for _, k := range keys {
		_ = m.setProfileComplete(ctx, k, complete) // expired/revoked keys are skipped
	}
}

func (m *Manager) setProfileComplete(ctx context.Context, key string, complete bool) error {
	raw, err := m.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return ErrNoSession
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return ErrNoSession
	}
	s.ProfileComplete = complete
	out, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// XX: never resurrect a session revoked between the GET and this SET.
	err = m.rdb.SetArgs(ctx, key, out, redis.SetArgs{Mode: "XX", KeepTTL: true}).Err()
	if errors.Is(err, redis.Nil) {
		return ErrNoSession
	}
	return err
}

// Destroy отзывает сессию (выход, сброс пароля, блокировка).
func (m *Manager) Destroy(ctx context.Context, token string) {
	if token == "" {
		return
	}
	if raw, err := m.rdb.Get(ctx, keyOf(token)).Bytes(); err == nil {
		var s Session
		if json.Unmarshal(raw, &s) == nil {
			_ = m.rdb.SRem(ctx, userIndex(s.UserID), keyOf(token)).Err()
		}
	}
	_ = m.rdb.Del(ctx, keyOf(token)).Err()
}

// DestroyByUser отзывает ВСЕ сессии пользователя (блокировка, сброс пароля).
func (m *Manager) DestroyByUser(ctx context.Context, userID int64) {
	keys, err := m.rdb.SMembers(ctx, userIndex(userID)).Result()
	if err != nil {
		return
	}
	if len(keys) > 0 {
		_ = m.rdb.Del(ctx, keys...).Err()
	}
	_ = m.rdb.Del(ctx, userIndex(userID)).Err()
}

func userIndex(userID int64) string { return "sess-by-user:" + itoa(userID) }

func (m *Manager) SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (m *Manager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (m *Manager) SetCsrfCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CsrfCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: false, // читается JS для заголовка — сверка на сервере
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func sessionTokenOf(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// SessionTokenOf достаёт токен сессии из куки запроса.
func SessionTokenOf(r *http.Request) string { return sessionTokenOf(r) }

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	b := []byte{}
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
