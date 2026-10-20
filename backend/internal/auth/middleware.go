package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ctxKey string

const sessionKey ctxKey = "session"

// UserOf достаёт сессию из контекста (кладёт LoadSession).
func UserOf(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionKey).(*Session)
	return s
}

// LoadSession — middleware: подгружает сессию в контекст, без неё пропускает.
func (m *Manager) LoadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s, err := m.Load(r.Context(), sessionTokenOf(r)); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), sessionKey, s))
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth отклоняет неаутентифицированных. Текст общий, без деталей.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserOf(r.Context()) == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ProfileLookup reports whether the user's avatar is set (avatar_set_at IS NOT NULL).
type ProfileLookup func(ctx context.Context, userID int64) (bool, error)

// RequireProfileComplete is the avatar gate. A session flagged complete passes
// for free. Otherwise the DB is re-read: if the avatar was set meanwhile (e.g. in
// another browser), this session is upgraded in place and the request passes;
// if not, 403 {"error":"avatar_required"}.
func (m *Manager) RequireProfileComplete(lookup ProfileLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := UserOf(r.Context())
			if s == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if !s.ProfileComplete {
				done, err := lookup(r.Context(), s.UserID)
				if err != nil {
					http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
					return
				}
				if !done {
					http.Error(w, `{"error":"avatar_required"}`, http.StatusForbidden)
					return
				}
				// Best effort: on failure the next request simply re-reads again.
				_ = m.UpdateProfileComplete(r.Context(), sessionTokenOf(r), true)
				s.ProfileComplete = true
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole отклоняет всех, чья серверная роль не входит в список.
// 401 — нет сессии, 403 — есть сессия, но роли не хватает.
func (m *Manager) RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := UserOf(r.Context())
			if s == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if !allowed[s.Role] {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireCSRF сверяет заголовок X-CSRF-Token с csrf-кукой (double-submit).
// GET/HEAD/OPTIONS не проверяются. Пути входа/регистрации исключены отдельно.
func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		header := r.Header.Get(CsrfHeader)
		cookie, err := r.Cookie(CsrfCookie)
		if err != nil || header == "" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) != 1 {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Limiter — per-IP token bucket для точек входа (защита от перебора).
// Idle visitors are swept so the map cannot grow without bound.
type Limiter struct {
	mu        sync.Mutex
	visitors  map[string]*visitor
	r         rate.Limit
	burst     int
	trusted   []netip.Prefix
	ttl       time.Duration
	lastSweep time.Time
	now       func() time.Time
}

type visitor struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// minVisitorTTL is the shortest idle time after which a visitor is forgotten.
const minVisitorTTL = 10 * time.Minute

// NewLimiter builds a per-client limiter. trusted are the proxy CIDRs whose
// X-Forwarded-For is honoured (see ClientIP).
func NewLimiter(rps rate.Limit, burst int, trusted []netip.Prefix) *Limiter {
	// An entry idle for at least the full-refill time is a fresh bucket, so
	// evicting it never relaxes the limit.
	ttl := minVisitorTTL
	if rps > 0 {
		if refill := time.Duration(float64(burst) / float64(rps) * float64(time.Second)); refill > ttl {
			ttl = refill
		}
	}
	return &Limiter{visitors: map[string]*visitor{}, r: rps, burst: burst, trusted: trusted, ttl: ttl, now: time.Now}
}

func (l *Limiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) >= l.ttl {
		for k, v := range l.visitors {
			if now.Sub(v.lastSeen) >= l.ttl {
				delete(l.visitors, k)
			}
		}
		l.lastSweep = now
	}
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{lim: rate.NewLimiter(l.r, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = now
	return v.lim
}

// Limit отклоняет превышение с 429 и Retry-After.
func (l *Limiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.get(ClientIP(r, l.trusted)).Allow() {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
