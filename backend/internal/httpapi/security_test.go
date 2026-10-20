// Security hardening tests: headers, health body, client IP / lockout keyed by
// login+IP, the last-admin race and the admin list cap. Infra-free tests run
// always; the rest need Postgres + Redis (see testDeps).
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"bigroup-backend/internal/admin"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

func TestSecurityHeaders(t *testing.T) {
	srv := offlineServer()
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/no-such-route"},
		{"OPTIONS", "/api/auth/login"},
		{"GET", "/api/auth/me"}, // 401 from RequireAuth
	} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		h := rec.Header()
		for name, want := range map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"Referrer-Policy":         "no-referrer",
			"Content-Security-Policy": apiCSP,
		} {
			if got := h.Get(name); got != want {
				t.Errorf("%s %s: %s = %q, want %q", tc.method, tc.path, name, got, want)
			}
		}
		if !strings.Contains(h.Get("Permissions-Policy"), "camera=()") {
			t.Errorf("%s %s: Permissions-Policy = %q", tc.method, tc.path, h.Get("Permissions-Policy"))
		}
		if !strings.Contains(apiCSP, "default-src 'none'") || !strings.Contains(apiCSP, "frame-ancestors 'none'") {
			t.Errorf("CSP not deny-all: %q", apiCSP)
		}
	}
}

func TestHealth_NoErrorDetails(t *testing.T) {
	srv, pool, rdb := testDeps(t)
	rec := do(t, srv, &jar{}, "GET", "/api/health", nil, "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("healthy: %d %s", rec.Code, rec.Body.String())
	}
	// Break both dependencies: the body must still carry only the status.
	pool.Close()
	_ = rdb.Close()
	rec = do(t, srv, &jar{}, "GET", "/api/health", nil, "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"degraded"}` {
		t.Fatalf("degraded: %d %s", rec.Code, rec.Body.String())
	}
}

// loginFrom posts a login with an explicit TCP peer and optional spoofed XFF.
func loginFrom(srv *Server, remote, xff, login, password string) int {
	raw, _ := json.Marshal(dto.LoginRequest{Login: login, Password: password})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(raw))
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec.Code
}

func TestLoginLockout_PerLoginAndIP(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	createUser(t, srv, "victim", "secret-123", model.RoleEmployee)

	// Attacker at 203.0.113.9 rotates spoofed XFF values: they are ignored, so the
	// attacker is locked as one source after loginFailLimit failures.
	for i := 0; i < loginFailLimit; i++ {
		if c := loginFrom(srv, "203.0.113.9:4000", fmt.Sprintf("1.2.3.%d", i), "victim", "wrong-pass"); c != 401 {
			t.Fatalf("attempt %d: %d, want 401", i, c)
		}
	}
	if c := loginFrom(srv, "203.0.113.9:4000", "9.9.9.9", "victim", "secret-123"); c != 429 {
		t.Fatalf("attacker IP after %d failures: %d, want 429", loginFailLimit, c)
	}
	// The victim from their own address is not locked out.
	if c := loginFrom(srv, "198.51.100.20:5000", "", "victim", "secret-123"); c != 200 {
		t.Fatalf("victim from another IP: %d, want 200", c)
	}
}

func TestLoginLockout_GlobalCeiling(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	createUser(t, srv, "target", "secret-123", model.RoleEmployee)

	// Distributed guessing: each IP stays under its own limit... until the
	// per-login ceiling across all IPs is reached.
	attempts := 0
	for ip := 1; attempts < loginGlobalFailLimit; ip++ {
		for k := 0; k < loginFailLimit-1 && attempts < loginGlobalFailLimit; k++ {
			if c := loginFrom(srv, fmt.Sprintf("10.20.%d.%d:1", ip/250, ip%250+1), "", "target", "wrong-pass"); c != 401 {
				t.Fatalf("attempt %d: %d, want 401", attempts, c)
			}
			attempts++
		}
	}
	if c := loginFrom(srv, "198.51.100.30:1", "", "target", "secret-123"); c != 429 {
		t.Fatalf("after global ceiling: %d, want 429", c)
	}
}

func TestClientIP_TrustedProxyOnly(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	srv.trusted = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	createUser(t, srv, "proxied", "secret-123", model.RoleEmployee)

	// Behind the trusted proxy the forwarded client is the lockout key.
	for i := 0; i < loginFailLimit; i++ {
		loginFrom(srv, "10.0.0.2:1", "6.6.6.6, 203.0.113.50", "proxied", "wrong-pass")
	}
	if c := loginFrom(srv, "10.0.0.2:1", "203.0.113.50", "proxied", "secret-123"); c != 429 {
		t.Fatalf("forwarded client should be locked: %d", c)
	}
	if c := loginFrom(srv, "10.0.0.2:1", "203.0.113.51", "proxied", "secret-123"); c != 200 {
		t.Fatalf("other forwarded client should pass: %d", c)
	}
	// The audit log records the resolved client, not the spoofed left-most hop.
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE ip='6.6.6.6'`).Scan(&n)
	if n != 0 {
		t.Fatalf("spoofed hop recorded in audit_log %d times", n)
	}
}

// Two admins demoting (or blocking) each other at the same time: exactly one wins,
// the other gets last_admin (409), and one active admin always remains.
func TestAdmin_LastAdminRace(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	ctx := context.Background()
	for round := 0; round < 15; round++ {
		mustExecSQL(t, pool, `TRUNCATE audit_log, users RESTART IDENTITY CASCADE`)
		a := createUser(t, srv, fmt.Sprintf("adm-a-%d", round), "secret-123", model.RoleAdmin)
		b := createUser(t, srv, fmt.Sprintf("adm-b-%d", round), "secret-123", model.RoleAdmin)
		block := round%2 == 1
		var wg sync.WaitGroup
		errs := make([]error, 2)
		start := make(chan struct{})
		for i, id := range []int64{a.ID, b.ID} {
			wg.Add(1)
			go func(i int, id int64) {
				defer wg.Done()
				<-start
				if block {
					_, errs[i] = srv.admin.SetBlocked(ctx, id, true)
				} else {
					_, errs[i] = srv.admin.ChangeRole(ctx, id, model.RoleEmployee)
				}
			}(i, id)
		}
		close(start)
		wg.Wait()
		ok, last := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, admin.ErrLastAdmin):
				last++
			default:
				t.Fatalf("round %d: unexpected error %v", round, err)
			}
		}
		if ok != 1 || last != 1 {
			t.Fatalf("round %d (block=%v): ok=%d last_admin=%d, want 1/1", round, block, ok, last)
		}
		var admins int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role='admin' AND status='active'`).Scan(&admins)
		if admins != 1 {
			t.Fatalf("round %d: %d active admins, want 1", round, admins)
		}
	}
	// The repo error maps to 409 last_admin at the HTTP layer.
	rec := httptest.NewRecorder()
	adminErr(rec, admin.ErrLastAdmin)
	if rec.Code != http.StatusConflict || errCode(rec) != "last_admin" {
		t.Fatalf("adminErr: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdmin_UsersListLimitCap(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	aj, _ := signedIn(t, srv, pool, "boss", model.RoleAdmin)
	mustExecSQL(t, pool, `INSERT INTO users (login, pass_hash, full_name, role)
		SELECT 'bulk-' || g, 'x', 'Bulk ' || g, 'employee' FROM generate_series(1, 250) g`)
	rec := do(t, srv, aj, "GET", "/api/admin/users?limit=100000", nil, "")
	want(t, rec, 200)
	l := parse[dto.AdminUserList](t, rec)
	if len(l.Items) != adminListMaxLimit || l.Total != 251 {
		t.Fatalf("items=%d total=%d, want %d/251", len(l.Items), l.Total, adminListMaxLimit)
	}
}
