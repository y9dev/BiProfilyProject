// Route-table tests: deny-by-default registration through protected()/public().
// TestRouteTable and TestProtectedRejects* need no infrastructure. The gate
// tests need Postgres + Redis (see testDeps) and skip without them.
// Steps 3B-6B extend this file: the sweep below automatically covers every
// route they register.
package httpapi

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/users"
)

// contractRoutes is the frozen API surface (plan section 7, backend/README.md).
// A registered route outside this set fails TestRouteTable (no legacy routes).
var contractRoutes = map[string]bool{
	"GET /api/health":                             true,
	"GET /api/register/options":                   true,
	"GET /api/widget/{userId}":                    true,
	"POST /api/auth/register":                     true,
	"POST /api/auth/login":                        true,
	"POST /api/auth/logout":                       true,
	"GET /api/auth/me":                            true,
	"GET /api/avatar/catalog":                     true,
	"PUT /api/me/avatar":                          true,
	"GET /api/me/profile":                         true,
	"GET /api/assessments":                        true,
	"GET /api/assessments/{id}/take":              true,
	"POST /api/assessments/{id}/submit":           true,
	"GET /api/me/attempts/{id}":                   true,
	"GET /api/characteristics":                    true,
	"POST /api/characteristics":                   true,
	"PUT /api/characteristics/{id}":               true,
	"DELETE /api/characteristics/{id}":            true,
	"GET /api/departments":                        true,
	"POST /api/departments":                       true,
	"PUT /api/departments/{id}":                   true,
	"DELETE /api/departments/{id}":                true,
	"POST /api/departments/{id}/activate":         true,
	"GET /api/invitations":                        true,
	"POST /api/invitations":                       true,
	"PATCH /api/invitations/{id}":                 true,
	"POST /api/invitations/{id}/revoke":           true,
	"DELETE /api/invitations/{id}":                true,
	"GET /api/invitations/{id}/registrations":     true,
	"GET /api/manage/assessments":                 true,
	"POST /api/manage/assessments":                true,
	"GET /api/manage/assessments/{id}":            true,
	"PATCH /api/manage/assessments/{id}":          true,
	"PUT /api/manage/assessments/{id}/visibility": true,
	"PUT /api/manage/assessments/{id}/expiry":     true,
	"DELETE /api/manage/assessments/{id}":         true,
	"GET /api/admin/users":                        true,
	"PUT /api/admin/users/{id}/role":              true,
	"POST /api/admin/users/{id}/block":            true,
	"POST /api/admin/users/{id}/unblock":          true,
	"POST /api/admin/users/{id}/reset-password":   true,
}

var managerPrefixes = []string{"/api/characteristics", "/api/departments", "/api/invitations", "/api/manage/"}

func offlineServer() *Server {
	return New(Deps{Users: users.New(nil), Sessions: auth.NewManager(nil, false)})
}

func TestRouteTable(t *testing.T) {
	routes := offlineServer().Routes()
	if len(routes) == 0 {
		t.Fatal("no routes registered")
	}
	seen := map[string]bool{}
	for _, r := range routes {
		p := r.Pattern()
		if seen[p] {
			t.Errorf("%s registered twice", p)
		}
		seen[p] = true
		if !contractRoutes[p] {
			t.Errorf("%s is not in the frozen contract", p)
		}
		if r.Public {
			if !publicRoutes[p] {
				t.Errorf("%s is public but not in the public allowlist", p)
			}
			continue
		}
		if !r.AvatarGate && !avatarExemptRoutes[p] {
			t.Errorf("%s skips the avatar gate but is not in the exempt allowlist", p)
		}
		if r.AvatarGate && avatarExemptRoutes[p] {
			t.Errorf("%s is in the exempt allowlist but registered with the gate", p)
		}
		if r.Method != http.MethodGet && !r.CSRF {
			t.Errorf("%s is a mutation without CSRF", p)
		}
		if strings.HasPrefix(r.Path, "/api/admin/") && !slices.Equal(r.Roles, rolesAdmin) {
			t.Errorf("%s roles = %v, want %v", p, r.Roles, rolesAdmin)
		}
		for _, prefix := range managerPrefixes {
			if strings.HasPrefix(r.Path, prefix) && !slices.Equal(r.Roles, rolesManager) {
				t.Errorf("%s roles = %v, want %v", p, r.Roles, rolesManager)
			}
		}
	}
	for p := range publicRoutes {
		if !contractRoutes[p] {
			t.Errorf("public allowlist entry %s is not in the contract", p)
		}
	}
	for p := range avatarExemptRoutes {
		if !contractRoutes[p] {
			t.Errorf("exempt allowlist entry %s is not in the contract", p)
		}
	}
}

func TestProtectedRejectsUnlistedExemption(t *testing.T) {
	s := offlineServer()
	defer func() {
		if recover() == nil {
			t.Fatal("avatarExempt on a non-allowlisted route must panic")
		}
	}()
	s.protected("GET /api/me/profile", func(http.ResponseWriter, *http.Request) {}, routeOpts{avatarExempt: true})
}

func TestPublicRejectsUnlistedRoute(t *testing.T) {
	s := offlineServer()
	defer func() {
		if recover() == nil {
			t.Fatal("public() on a non-allowlisted route must panic")
		}
	}()
	s.public("GET /api/assessments", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
}

// signIn creates a session for u directly (no login round-trip) and returns a cookie jar.
func signIn(t *testing.T, srv *Server, u model.User) *jar {
	t.Helper()
	token, err := srv.sessions.Create(context.Background(), u.ID, u.Role, u.ProfileComplete())
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	return &jar{cookies: map[string]string{auth.SessionCookie: token, auth.CsrfCookie: "test-csrf"}}
}

func setAvatar(t *testing.T, srv *Server, userID int64) {
	t.Helper()
	if _, err := srv.pool.Exec(context.Background(), `UPDATE users SET avatar_universe='movies', avatar_gender='female',
		avatar_style='calm', avatar_set_at=now() WHERE id=$1`, userID); err != nil {
		t.Fatalf("set avatar: %v", err)
	}
}

func fillPath(path string) string {
	return strings.NewReplacer("{id}", "1").Replace(path)
}

// TestAvatarGateSweep: a signed-in user without an avatar gets 403 avatar_required
// on every registered non-exempt route (AC-3). Covers routes added by 3B-6B automatically.
func TestAvatarGateSweep(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	u := createUser(t, srv, "noavatar", "secret-123", model.RoleAdmin) // admin: role checks never mask the gate
	j := signIn(t, srv, u)
	for _, r := range srv.Routes() {
		if r.Public || !r.AvatarGate {
			continue
		}
		rec := do(t, srv, j, r.Method, fillPath(r.Path), map[string]any{}, csrfOf(j))
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "avatar_required") {
			t.Errorf("%s: %d %s, want 403 avatar_required", r.Pattern(), rec.Code, rec.Body.String())
		}
	}
	if rec := do(t, srv, j, "GET", "/api/auth/me", nil, ""); rec.Code != http.StatusOK {
		t.Errorf("GET /api/auth/me: %d, exempt route must pass without an avatar", rec.Code)
	}
}

// TestProtectedComposition exercises the helper on probe routes: auth, avatar gate
// with DB re-read and in-place session upgrade, role, CSRF.
func TestProtectedComposition(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	srv.protected("GET /api/probe/profile", ok, routeOpts{})
	srv.protected("GET /api/probe/mgr", ok, routeOpts{roles: rolesManager})
	srv.protected("POST /api/probe/mgr", ok, routeOpts{roles: rolesManager})

	emp := createUser(t, srv, "emp", "secret-123", model.RoleEmployee)
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	empJ := signIn(t, srv, emp) // session flag false
	mgrJ := signIn(t, srv, mgr)

	if rec := do(t, srv, &jar{}, "GET", "/api/probe/profile", nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d, want 401", rec.Code)
	}
	if rec := do(t, srv, empJ, "GET", "/api/probe/profile", nil, ""); rec.Code != http.StatusForbidden ||
		!strings.Contains(rec.Body.String(), "avatar_required") {
		t.Fatalf("no avatar: %d %s, want 403 avatar_required", rec.Code, rec.Body.String())
	}

	// Avatar set elsewhere: the stale session is upgraded in place, not blocked.
	setAvatar(t, srv, emp.ID)
	if rec := do(t, srv, empJ, "GET", "/api/probe/profile", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("after avatar: %d %s, want 200", rec.Code, rec.Body.String())
	}
	sess, err := srv.sessions.Load(context.Background(), empJ.cookies[auth.SessionCookie])
	if err != nil || !sess.ProfileComplete {
		t.Fatalf("session not upgraded in place: %+v %v", sess, err)
	}

	if rec := do(t, srv, empJ, "GET", "/api/probe/mgr", nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("employee on manager route: %d, want 403", rec.Code)
	}
	setAvatar(t, srv, mgr.ID)
	if rec := do(t, srv, mgrJ, "GET", "/api/probe/mgr", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("manager GET: %d, want 200", rec.Code)
	}
	if rec := do(t, srv, mgrJ, "POST", "/api/probe/mgr", nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("manager POST without csrf: %d, want 403", rec.Code)
	}
	if rec := do(t, srv, mgrJ, "POST", "/api/probe/mgr", nil, csrfOf(mgrJ)); rec.Code != http.StatusOK {
		t.Fatalf("manager POST with csrf: %d, want 200", rec.Code)
	}
}
