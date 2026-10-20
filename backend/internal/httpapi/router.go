// Package httpapi is the HTTP layer: routing, JSON, CORS.
// Routes are registered per area (registerAuthRoutes, registerEmployeeRoutes,
// registerManagerRoutes, registerAdminRoutes, registerDashboardRoutes,
// registerAnalyticsRoutes) through the protected()/public()
// helpers in routes.go, which apply auth, the avatar gate, roles and CSRF.
package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"bigroup-backend/internal/admin"
	"bigroup-backend/internal/analytics"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/invitations"
	"bigroup-backend/internal/users"
)

// Deps are the server's collaborators. Later steps add their repositories here.
type Deps struct {
	Pool        *pgxpool.Pool
	Redis       *redis.Client
	Users       users.Repository
	Admin       admin.Repository
	Invitations invitations.Repository
	Sessions    *auth.Manager
	FrontOrigin string
	// TrustedProxies are the proxy CIDRs whose X-Forwarded-For is honoured (auth.ClientIP).
	TrustedProxies []netip.Prefix
	// Analytics backs /api/dashboard/* and /api/analytics/*; nil = analytics.New(Pool, Redis).
	Analytics analytics.Repo
	// AnalyticsTimeout bounds each analytics request; 0 = analytics.QueryTimeout.
	AnalyticsTimeout time.Duration
}

type Server struct {
	mux              *http.ServeMux
	pool             *pgxpool.Pool
	rdb              *redis.Client
	users            users.Repository
	admin            admin.Repository
	invitations      invitations.Repository
	sessions         *auth.Manager
	loginLim         *auth.Limiter
	regLim           *auth.Limiter
	widgetLim        *auth.Limiter
	widgetProfile    func(ctx context.Context, userID int64) (dto.Profile, error)
	origin           string
	trusted          []netip.Prefix
	routes           []RouteMeta
	analytics        analytics.Repo
	analyticsTimeout time.Duration
}

func New(d Deps) *Server {
	s := &Server{
		mux: http.NewServeMux(), pool: d.Pool, rdb: d.Redis,
		users: d.Users, admin: d.Admin, invitations: d.Invitations, sessions: d.Sessions,
		loginLim:  auth.NewLimiter(5.0/60.0, 10, d.TrustedProxies),
		regLim:    auth.NewLimiter(5.0/60.0, 10, d.TrustedProxies),
		widgetLim: auth.NewLimiter(2, 30, d.TrustedProxies),
		origin:    d.FrontOrigin,
		trusted:   d.TrustedProxies,
		analytics: d.Analytics, analyticsTimeout: d.AnalyticsTimeout,
	}
	if s.analytics == nil {
		s.analytics = analytics.New(d.Pool, d.Redis)
	}
	if s.analyticsTimeout <= 0 {
		s.analyticsTimeout = analytics.QueryTimeout
	}
	s.public("GET /api/health", http.HandlerFunc(s.health))
	s.registerAuthRoutes()
	s.registerWidgetRoutes()
	s.registerEmployeeRoutes()
	s.registerManagerRoutes()
	s.registerAdminRoutes()
	s.registerDashboardRoutes()
	s.registerAnalyticsRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.sessions.LoadSession(s.cors(logRequests(s.mux))))
}

// apiCSP is a deny-all policy: the API serves JSON only, never documents or scripts.
const apiCSP = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// securityHeaders sets defensive headers on every response, including errors and preflights.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", apiCSP)
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Куки авторизации требуют точный origin, не "*".
		w.Header().Set("Access-Control-Allow-Origin", s.origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, Idempotency-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	// Only the status is public; the causes go to the log (they can leak hosts, users).
	status := "ok"
	if err := s.pool.Ping(ctx); err != nil {
		status = "degraded"
		log.Printf("health: postgres: %v", err)
	}
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		status = "degraded"
		log.Printf("health: redis: %v", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}
