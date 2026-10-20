package httpapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/model"
)

// Deny by default: every /api route is registered through public() or protected().
// Direct s.mux.Handle for /api/* is forbidden; routes_test.go checks the registry.

// publicRoutes is the exhaustive allowlist of unauthenticated routes.
var publicRoutes = map[string]bool{
	"GET /api/health":           true,
	"POST /api/auth/login":      true,
	"POST /api/auth/register":   true,
	"GET /api/register/options": true,
	"GET /api/widget/{userId}":  true,
}

// avatarExemptRoutes is the exhaustive allowlist of signed-in routes that skip the avatar gate.
var avatarExemptRoutes = map[string]bool{
	"GET /api/auth/me":        true,
	"POST /api/auth/logout":   true,
	"PUT /api/me/avatar":      true,
	"GET /api/avatar/catalog": true,
}

// Role sets for routeOpts.roles.
var (
	rolesManager = []string{model.RoleManager, model.RoleAdmin}
	rolesAdmin   = []string{model.RoleAdmin}
)

// routeOpts configures protected(). CSRF is not optional: it is derived from the
// method (every non-GET route is checked).
type routeOpts struct {
	roles        []string // empty = any signed-in user
	avatarExempt bool     // must be listed in avatarExemptRoutes
}

// RouteMeta records how a route was registered (read by routes_test.go).
type RouteMeta struct {
	Method     string
	Path       string
	Public     bool
	AvatarGate bool
	Roles      []string
	CSRF       bool
}

// Pattern is the ServeMux pattern, e.g. "GET /api/auth/me".
func (m RouteMeta) Pattern() string { return m.Method + " " + m.Path }

// Routes returns a copy of the route metadata registry.
func (s *Server) Routes() []RouteMeta { return slices.Clone(s.routes) }

func splitPattern(pattern string) (method, path string) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok || method == "" || !strings.HasPrefix(path, "/") {
		panic(fmt.Sprintf("route %q: pattern must be \"METHOD /path\"", pattern))
	}
	return method, path
}

// public registers an unauthenticated route; only patterns in publicRoutes are accepted.
func (s *Server) public(pattern string, h http.Handler) {
	method, path := splitPattern(pattern)
	if !publicRoutes[pattern] {
		panic(fmt.Sprintf("route %q is not in the public allowlist", pattern))
	}
	s.routes = append(s.routes, RouteMeta{Method: method, Path: path, Public: true})
	s.mux.Handle(pattern, h)
}

// protected registers an authenticated route composed as
// RequireAuth -> RequireProfileComplete (unless avatarExempt) -> RequireRole -> RequireCSRF (non-GET).
func (s *Server) protected(pattern string, h http.HandlerFunc, o routeOpts) {
	method, path := splitPattern(pattern)
	if o.avatarExempt && !avatarExemptRoutes[pattern] {
		panic(fmt.Sprintf("route %q is not in the avatar-exempt allowlist", pattern))
	}
	if publicRoutes[pattern] {
		panic(fmt.Sprintf("route %q is public and cannot be protected", pattern))
	}
	meta := RouteMeta{Method: method, Path: path, AvatarGate: !o.avatarExempt, Roles: slices.Clone(o.roles)}

	var next http.Handler = h
	if method != http.MethodGet && method != http.MethodHead {
		next = auth.RequireCSRF(next)
		meta.CSRF = true
	}
	if len(o.roles) > 0 {
		next = s.sessions.RequireRole(o.roles...)(next)
	}
	if !o.avatarExempt {
		next = s.sessions.RequireProfileComplete(func(ctx context.Context, userID int64) (bool, error) {
			return s.users.ProfileComplete(ctx, userID)
		})(next)
	}
	next = auth.RequireAuth(next)

	s.routes = append(s.routes, meta)
	s.mux.Handle(pattern, next)
}

// statusRecorder captures the response status for the request log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// logRequests writes one line per request: method, path, status, duration.
// No bodies, query strings or headers are logged (they may carry secrets).
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		// A client that aborts (React StrictMode unmount, navigation) cancels r.Context();
		// pgx/redis then fail and the handler writes 500/401 that nobody receives. Mark it,
		// so a canceled request is not mistaken for a server fault.
		note := ""
		if r.Context().Err() != nil {
			note = " client canceled"
		}
		log.Printf("%s %s -> %d (%s)%s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond), note)
	})
}
