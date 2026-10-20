// Package httpapi — auth-эндпоинты среза identity.
// Правила: роль только из серверной сессии; ответы об ошибках входа
// одинаковые для «нет такого кода» и «неверный пароль» (anti-enumeration).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

// genericLoginErr — единый ответ входа, чтобы не перечислять пользователей.
const genericLoginErr = `{"error":"invalid login or password"}`

var (
	dummyHash     string
	dummyHashOnce sync.Once
)

func getDummyHash() string {
	dummyHashOnce.Do(func() {
		h, err := auth.HashPassword("dummy-password-for-timing-parity-0123456789")
		if err == nil {
			dummyHash = h
		}
	})
	return dummyHash
}

// clientIPOf is the client address for lockout and audit (see auth.ClientIP).
func (s *Server) clientIPOf(r *http.Request) string { return auth.ClientIP(r, s.trusted) }

func (s *Server) audit(r *http.Request, e model.AuditEntry) {
	e.IP = s.clientIPOf(r)
	_ = auth.WriteAudit(r.Context(), s.pool, e)
}

// Lockout. Failures are counted per login+IP, so an attacker elsewhere cannot lock
// the victim out of their own address. A much higher per-login ceiling across all
// IPs caps distributed guessing without letting one source lock the account.
const (
	// loginFailLimit — неудачных попыток с одного IP для одного логина до блокировки.
	loginFailLimit = 5
	// loginGlobalFailLimit — неудачных попыток для логина со всех IP за окно.
	loginGlobalFailLimit = 50
	// loginLockTTL — окно подсчёта и длительность блокировки.
	loginLockTTL = 15 * time.Minute
)

func loginFailKey(login, ip string) string   { return "login_failures:" + login + "|" + ip }
func loginLockKey(login, ip string) string   { return "login_lock:" + login + "|" + ip }
func loginGlobalFailKey(login string) string { return "login_failures_all:" + login }
func loginGlobalLockKey(login string) string { return "login_lock_all:" + login }

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, genericLoginErr, http.StatusUnauthorized)
		return
	}
	in.Login = strings.ToLower(strings.TrimSpace(in.Login))
	if in.Login == "" || len(in.Login) > 64 || in.Password == "" {
		http.Error(w, genericLoginErr, http.StatusUnauthorized)
		return
	}

	// Проверяем, не заблокирован ли аккаунт из-за превышения лимита неудачных попыток.
	ip := s.clientIPOf(r)
	if n, err := s.rdb.Exists(r.Context(), loginLockKey(in.Login, ip), loginGlobalLockKey(in.Login)).Result(); err == nil && n > 0 {
		s.audit(r, model.AuditEntry{Action: "login_locked", EntityType: "user", EntityID: in.Login, Result: "deny"})
		http.Error(w, `{"error":"account temporarily locked, try again later"}`, http.StatusTooManyRequests)
		return
	}

	u, hash, err := s.users.GetByLogin(r.Context(), in.Login)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Выравниваем время ответа: хешируем всегда.
			_, _ = auth.VerifyPassword(in.Password, getDummyHash())
			s.incrementLoginFailures(r.Context(), in.Login, ip)
			s.audit(r, model.AuditEntry{Action: "login_fail", EntityType: "user", EntityID: in.Login, Result: "fail"})
			http.Error(w, genericLoginErr, http.StatusUnauthorized)
			return
		}
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	ok, err := auth.VerifyPassword(in.Password, hash)
	if err != nil || !ok {
		s.incrementLoginFailures(r.Context(), in.Login, ip)
		s.audit(r, model.AuditEntry{ActorID: &u.ID, ActorRole: u.Role, Action: "login_fail", EntityType: "user", EntityID: itoa(u.ID), Result: "fail"})
		http.Error(w, genericLoginErr, http.StatusUnauthorized)
		return
	}
	if u.Status != model.UserActive {
		s.audit(r, model.AuditEntry{ActorID: &u.ID, ActorRole: u.Role, Action: "login_blocked", EntityType: "user", EntityID: itoa(u.ID), Result: "deny"})
		http.Error(w, `{"error":"account unavailable"}`, http.StatusForbidden)
		return
	}
	// Успешный вход: сбрасываем счётчик неудачных попыток.
	// The global counter is kept: it expires with its window, so a victim's own login
	// does not reset an ongoing distributed attack.
	s.rdb.Del(r.Context(), loginFailKey(in.Login, ip))

	// Ротация: гасим предыдущую сессию с этого браузера, выдаём новую.
	s.sessions.Destroy(r.Context(), auth.SessionTokenOf(r))
	token, err := s.sessions.Create(r.Context(), u.ID, u.Role, u.ProfileComplete())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	csrf, err := newCsrf()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	s.sessions.SetSessionCookie(w, token)
	s.sessions.SetCsrfCookie(w, csrf)
	s.audit(r, model.AuditEntry{ActorID: &u.ID, ActorRole: u.Role, Action: "login_ok", EntityType: "user", EntityID: itoa(u.ID), Result: "ok"})
	writeJSON(w, http.StatusOK, meOf(u))
}

// incrementLoginFailures counts a failure per login+IP and per login, and sets the
// matching lock once a limit is reached.
func (s *Server) incrementLoginFailures(ctx context.Context, login, ip string) {
	if s.bumpFailures(ctx, loginFailKey(login, ip)) >= loginFailLimit {
		s.rdb.Set(ctx, loginLockKey(login, ip), "1", loginLockTTL)
	}
	if s.bumpFailures(ctx, loginGlobalFailKey(login)) >= loginGlobalFailLimit {
		s.rdb.Set(ctx, loginGlobalLockKey(login), "1", loginLockTTL)
	}
}

// bumpFailures increments a fixed-window counter; 0 on Redis error.
func (s *Server) bumpFailures(ctx context.Context, key string) int64 {
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0
	}
	if n == 1 {
		s.rdb.Expire(ctx, key, loginLockTTL)
	}
	return n
}

func newCsrf() (string, error) {
	// csrf-токен генерируется внутри auth при создании сессии? Нет —
	// отдельная функция здесь, чтобы не раздувать Manager.
	return auth.NewCsrfToken()
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	sess := auth.UserOf(r.Context())
	s.sessions.Destroy(r.Context(), auth.SessionTokenOf(r))
	s.sessions.ClearSessionCookie(w)
	if sess != nil {
		s.audit(r, model.AuditEntry{ActorID: &sess.UserID, ActorRole: sess.Role, Action: "logout", Result: "ok"})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	sess := auth.UserOf(r.Context())
	// Свежие данные из БД: роль/блокировку могли изменить после входа.
	u, err := s.users.GetByID(r.Context(), sess.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	if u.Status != model.UserActive {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, meOf(u))
}

// deptRef builds the embedded department, nil when the user has none.
func deptRef(id *int64, name *string) *dto.DepartmentRef {
	if id == nil || name == nil {
		return nil
	}
	return &dto.DepartmentRef{ID: *id, Name: *name}
}

// meOf maps a user to the frozen GET /api/auth/me shape.
func meOf(u model.User) dto.Me {
	out := dto.Me{
		ID: u.ID, Login: u.Login, FullName: u.FullName, JobTitle: u.JobTitle,
		Department: deptRef(u.DepartmentID, u.DepartmentName), Role: u.Role,
		ProfileComplete: u.ProfileComplete(),
	}
	if u.Avatar != nil {
		out.Avatar = &dto.Avatar{Universe: u.Avatar.Universe, Gender: u.Avatar.Gender, Style: u.Avatar.Style}
	}
	return out
}

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

// registerAuthRoutes: registration, sign-in, sign-out, the current user and the avatar step.
func (s *Server) registerAuthRoutes() {
	s.public("GET /api/register/options", s.regLim.Limit(http.HandlerFunc(s.registerOptions)))
	s.public("POST /api/auth/register", s.regLim.Limit(http.HandlerFunc(s.register)))
	s.public("POST /api/auth/login", s.loginLim.Limit(http.HandlerFunc(s.login)))
	s.protected("GET /api/avatar/catalog", s.avatarCatalog, routeOpts{avatarExempt: true})
	s.protected("PUT /api/me/avatar", s.setMyAvatar, routeOpts{avatarExempt: true})
	s.protected("POST /api/auth/logout", s.logout, routeOpts{avatarExempt: true})
	s.protected("GET /api/auth/me", s.me, routeOpts{avatarExempt: true})
}
