// Integration tests for sign-in, sign-out and the session. They need Postgres
// and Redis (TEST_PG_DSN / TEST_REDIS_ADDR, localhost defaults) and skip with
// an explanation when either is unreachable. The database is migrated to the
// new baseline and all data tables are truncated before each test.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"bigroup-backend/internal/admin"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/migrate"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/testutil"
	"bigroup-backend/internal/users"
)

func testDeps(t *testing.T) (*Server, *pgxpool.Pool, *redis.Client) {
	t.Helper()
	pgDSN := os.Getenv("TEST_PG_DSN")
	if pgDSN == "" {
		pgDSN = "postgres://postgres:postgres@localhost:5432/bigroup?sslmode=disable"
	}
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, pgDSN)
	if err != nil {
		t.Skipf("no postgres (%v)", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("no postgres (%v)", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		pool.Close()
		t.Skipf("no redis (%v)", err)
	}
	if _, err := migrate.Up(context.Background(), pool); err != nil {
		pool.Close()
		t.Fatalf("migrate: %v", err)
	}
	// Isolate the run: wipe data, keep schema and the seeded default characteristics.
	if _, err := pool.Exec(context.Background(), `
		TRUNCATE audit_log, attempt_scores, attempt_answers, attempts, option_effects, options, questions,
		         assessments, users, invitations, departments RESTART IDENTITY CASCADE;
		DELETE FROM characteristics WHERE NOT is_default;`); err != nil {
		pool.Close()
		t.Fatalf("cleanup: %v", err)
	}
	_ = rdb.FlushDB(context.Background()).Err()
	srv := New(Deps{
		Pool: pool, Redis: rdb, Users: users.New(pool), Admin: admin.New(pool),
		Sessions: auth.NewManager(rdb, false), FrontOrigin: "http://localhost:5173",
	})
	t.Cleanup(func() { _ = rdb.Close() })
	return srv, pool, rdb
}

type jar struct{ cookies map[string]string }

func (j *jar) apply(resp *http.Response) {
	if j.cookies == nil {
		j.cookies = map[string]string{}
	}
	for _, c := range resp.Cookies() {
		j.cookies[c.Name] = c.Value
	}
}

func do(t *testing.T, srv *Server, j *jar, method, path string, body any, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	for name, val := range j.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: val})
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	j.apply(rec.Result())
	return rec
}

func csrfOf(j *jar) string { return j.cookies["bi_csrf"] }

// createUser inserts a user directly (no invitation) for tests.
func createUser(t *testing.T, srv *Server, login, password, role string) model.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := srv.users.Create(context.Background(), model.User{Login: login, FullName: "Тест " + login, Role: role}, hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestLoginLogoutMe(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	createUser(t, srv, "t-1", "secret-123", model.RoleEmployee)

	j := &jar{}
	rec := do(t, srv, j, "POST", "/api/auth/login", dto.LoginRequest{Login: "T-1", Password: "secret-123"}, "")
	if rec.Code != 200 {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	if j.cookies["bi_sid"] == "" || j.cookies["bi_csrf"] == "" {
		t.Fatal("no session/csrf cookies")
	}
	rec = do(t, srv, j, "GET", "/api/auth/me", nil, "")
	if rec.Code != 200 {
		t.Fatalf("me: %d %s", rec.Code, rec.Body.String())
	}
	var me dto.Me
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatalf("me json: %v", err)
	}
	if me.Login != "t-1" || me.ProfileComplete || me.Avatar != nil || me.Department != nil {
		t.Fatalf("me: %+v", me)
	}

	rec = do(t, srv, j, "POST", "/api/auth/logout", nil, csrfOf(j))
	if rec.Code != 204 {
		t.Fatalf("logout: %d", rec.Code)
	}
	rec = do(t, srv, j, "GET", "/api/auth/me", nil, "")
	if rec.Code != 401 {
		t.Fatalf("me after logout: %d, want 401", rec.Code)
	}
}

func TestLoginGenericErrors(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	createUser(t, srv, "t-2", "secret-123", model.RoleEmployee)

	j := &jar{}
	badPass := do(t, srv, j, "POST", "/api/auth/login", dto.LoginRequest{Login: "t-2", Password: "nope-nope"}, "")
	noUser := do(t, srv, j, "POST", "/api/auth/login", dto.LoginRequest{Login: "no-such", Password: "nope-nope"}, "")
	if badPass.Code != 401 || noUser.Code != 401 {
		t.Fatalf("codes: %d %d, want 401 401", badPass.Code, noUser.Code)
	}
	if badPass.Body.String() != noUser.Body.String() {
		t.Fatalf("bodies differ (enumeration): %q vs %q", badPass.Body.String(), noUser.Body.String())
	}
}

func TestBlockedDeniedEverywhere(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	ctx := context.Background()
	u := createUser(t, srv, "t-3", "secret-123", model.RoleEmployee)

	j := &jar{}
	_ = do(t, srv, j, "POST", "/api/auth/login", dto.LoginRequest{Login: "t-3", Password: "secret-123"}, "")
	if j.cookies["bi_sid"] == "" {
		t.Fatal("login failed")
	}
	_ = srv.users.SetStatus(ctx, u.ID, model.UserBlocked)
	srv.sessions.DestroyByUser(ctx, u.ID)

	rec := do(t, srv, j, "GET", "/api/auth/me", nil, "")
	if rec.Code != 401 && rec.Code != 403 {
		t.Fatalf("blocked session still works: %d", rec.Code)
	}
	rec = do(t, srv, j, "POST", "/api/auth/login", dto.LoginRequest{Login: "t-3", Password: "secret-123"}, "")
	if rec.Code != 403 {
		t.Fatalf("blocked login: %d, want 403", rec.Code)
	}
}

func TestCsrfRequired(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	createUser(t, srv, "t-4", "secret-123", model.RoleAdmin)
	j := &jar{}
	_ = do(t, srv, j, "POST", "/api/auth/login", dto.LoginRequest{Login: "t-4", Password: "secret-123"}, "")

	// A mutation without the CSRF header is rejected even with a valid session.
	rec := do(t, srv, j, "POST", "/api/auth/logout", nil, "")
	if rec.Code != 403 {
		t.Fatalf("logout without csrf: %d, want 403", rec.Code)
	}
}

// ---------- registration, options, avatar (Step 3B) ----------

func regBody(code, login string) dto.RegisterRequest {
	return dto.RegisterRequest{Code: code, Login: login, Password: "secret-123", FullName: "Иван Иванов", JobTitle: "Инженер"}
}

func inviteCount(t *testing.T, pool *pgxpool.Pool, id int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT registrations_count FROM invitations WHERE id=$1`, id).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestRegister_HappyPath(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	adm := testutil.User(t, pool, "boss", "secret-123", model.RoleAdmin, true)
	invID := testutil.Invitation(t, pool, adm.ID, "ABCDEFGHJKLM", testutil.InviteOpts{Max: 2})
	dep := testutil.Department(t, pool, "Продажи", true)

	j := &jar{}
	body := regBody("abcdefghjklm", "New.User") // code is case-insensitive
	body.DepartmentID = &dep
	rec := do(t, srv, j, "POST", "/api/auth/register", body, "")
	if rec.Code != 201 {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	var me dto.Me
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Role != model.RoleEmployee || me.ProfileComplete || me.Department == nil || me.Department.ID != dep {
		t.Fatalf("me: %+v", me)
	}
	if j.cookies["bi_sid"] == "" || j.cookies["bi_csrf"] == "" {
		t.Fatal("no session cookies after register")
	}
	if n := inviteCount(t, pool, invID); n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	if rec := do(t, srv, j, "GET", "/api/auth/me", nil, ""); rec.Code != 200 {
		t.Fatalf("me: %d", rec.Code)
	}
}

func TestRegister_InvalidInvites(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	adm := testutil.User(t, pool, "boss", "secret-123", model.RoleAdmin, true)
	past := time.Now().Add(-time.Hour)
	testutil.Invitation(t, pool, adm.ID, "EXPIREDEXPIR", testutil.InviteOpts{ExpiresAt: &past})
	testutil.Invitation(t, pool, adm.ID, "FULLFULLFULL", testutil.InviteOpts{Max: 1, Count: 1})
	testutil.Invitation(t, pool, adm.ID, "REVOKEDREVOK", testutil.InviteOpts{Revoked: true})
	var first string
	for i, code := range []string{"NOSUCHCODE22", "EXPIREDEXPIR", "FULLFULLFULL", "REVOKEDREVOK"} {
		rec := do(t, srv, &jar{}, "POST", "/api/auth/register", regBody(code, "user"+string(rune('a'+i))), "")
		if rec.Code != 410 || !strings.Contains(rec.Body.String(), `"invite_invalid"`) {
			t.Fatalf("%s: %d %s, want 410 invite_invalid", code, rec.Code, rec.Body.String())
		}
		if first == "" {
			first = rec.Body.String()
		} else if rec.Body.String() != first {
			t.Fatalf("%s: reason leaks (%s vs %s)", code, rec.Body.String(), first)
		}
	}
}

func TestRegister_LoginTakenRestoresCount(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	adm := testutil.User(t, pool, "boss", "secret-123", model.RoleAdmin, true)
	invID := testutil.Invitation(t, pool, adm.ID, "TAKENTAKENTA", testutil.InviteOpts{Max: 3})
	if rec := do(t, srv, &jar{}, "POST", "/api/auth/register", regBody("TAKENTAKENTA", "dup"), ""); rec.Code != 201 {
		t.Fatalf("first: %d %s", rec.Code, rec.Body.String())
	}
	rec := do(t, srv, &jar{}, "POST", "/api/auth/register", regBody("TAKENTAKENTA", "DUP"), "") // citext: same login
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "login_taken") {
		t.Fatalf("dup: %d %s, want 409 login_taken", rec.Code, rec.Body.String())
	}
	if n := inviteCount(t, pool, invID); n != 1 {
		t.Fatalf("count = %d, want 1 (rollback must restore the slot)", n)
	}
}

func TestRegister_Validation(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	adm := testutil.User(t, pool, "boss", "secret-123", model.RoleAdmin, true)
	invID := testutil.Invitation(t, pool, adm.ID, "VALIDVALIDVA", testutil.InviteOpts{})
	inactive := testutil.Department(t, pool, "Старый", false)

	rec := do(t, srv, &jar{}, "POST", "/api/auth/register", dto.RegisterRequest{Code: "VALIDVALIDVA", Login: "a b", Password: "short"}, "")
	var e dto.Error
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	paths := map[string]string{}
	for _, f := range e.Errors {
		paths[f.Path] = f.Code
	}
	if rec.Code != 422 || paths["login"] != "invalid" || paths["password"] != "too_short" || paths["fullName"] != "required" {
		t.Fatalf("validation: %d %s", rec.Code, rec.Body.String())
	}
	body := regBody("VALIDVALIDVA", "okuser")
	body.DepartmentID = &inactive
	if rec := do(t, srv, &jar{}, "POST", "/api/auth/register", body, ""); rec.Code != 422 || !strings.Contains(rec.Body.String(), "departmentId") {
		t.Fatalf("inactive dept: %d %s", rec.Code, rec.Body.String())
	}
	if n := inviteCount(t, pool, invID); n != 0 {
		t.Fatalf("failed registrations consumed the invite: %d", n)
	}
}

func TestRegisterOptions(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	adm := testutil.User(t, pool, "boss", "secret-123", model.RoleAdmin, true)
	testutil.Invitation(t, pool, adm.ID, "OPTSOPTSOPTS", testutil.InviteOpts{})
	testutil.Department(t, pool, "Производство", true)
	testutil.Department(t, pool, "Закрыт", false)

	rec := do(t, srv, &jar{}, "GET", "/api/register/options?code=optsoptsopts", nil, "")
	var o dto.RegisterOptions
	_ = json.Unmarshal(rec.Body.Bytes(), &o)
	if rec.Code != 200 || !o.Valid || len(o.Departments) != 1 || o.Departments[0].Name != "Производство" {
		t.Fatalf("valid: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, srv, &jar{}, "GET", "/api/register/options?code=WRONGWRONGWR", nil, "")
	o = dto.RegisterOptions{}
	_ = json.Unmarshal(rec.Body.Bytes(), &o)
	if rec.Code != 200 || o.Valid || o.Reason != "invite_invalid" || len(o.Departments) != 0 {
		t.Fatalf("invalid: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAvatar_CatalogAndPut(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	u := createUser(t, srv, "avatar-u", "secret-123", model.RoleEmployee)
	j := signIn(t, srv, u)

	if rec := do(t, srv, j, "GET", "/api/avatar/catalog", nil, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"universes"`) {
		t.Fatalf("catalog: %d %s", rec.Code, rec.Body.String())
	}
	bad := dto.Avatar{Universe: "nope", Gender: "male", Style: "tech"}
	if rec := do(t, srv, j, "PUT", "/api/me/avatar", bad, csrfOf(j)); rec.Code != 422 {
		t.Fatalf("bad avatar: %d, want 422", rec.Code)
	}
	if rec := do(t, srv, j, "PUT", "/api/me/avatar", dto.Avatar{Universe: "anime", Gender: "male", Style: "tech"}, ""); rec.Code != 403 {
		t.Fatalf("no csrf: %d, want 403", rec.Code)
	}
	rec := do(t, srv, j, "PUT", "/api/me/avatar", dto.Avatar{Universe: "anime", Gender: "male", Style: "tech"}, csrfOf(j))
	var me dto.Me
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if rec.Code != 200 || !me.ProfileComplete || me.Avatar == nil || me.Avatar.Universe != "anime" {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
}

// TestAvatarGate_TwoSessions (plan section 6): session B is created before the
// avatar is set in session A; B must not be blocked afterwards.
func TestAvatarGate_TwoSessions(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	adm := testutil.User(t, pool, "boss", "secret-123", model.RoleAdmin, true)
	testutil.Invitation(t, pool, adm.ID, "TWOSESSTWOSE", testutil.InviteOpts{})

	a := &jar{}
	if rec := do(t, srv, a, "POST", "/api/auth/register", regBody("TWOSESSTWOSE", "two.sessions"), ""); rec.Code != 201 {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	b := &jar{}
	if rec := do(t, srv, b, "POST", "/api/auth/login", dto.LoginRequest{Login: "two.sessions", Password: "secret-123"}, ""); rec.Code != 200 {
		t.Fatalf("login B: %d", rec.Code)
	}
	srv.protected("GET /api/probe/gated", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }, routeOpts{})
	gated := func(j *jar) *httptest.ResponseRecorder {
		// Probe route registered through protected(): same gate as every real route.
		return do(t, srv, j, "GET", "/api/probe/gated", nil, "")
	}
	if rec := gated(b); rec.Code != 403 || !strings.Contains(rec.Body.String(), "avatar_required") {
		t.Fatalf("B before avatar: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, srv, a, "PUT", "/api/me/avatar", dto.Avatar{Universe: "sports", Gender: "female", Style: "vibrant"}, csrfOf(a)); rec.Code != 200 {
		t.Fatalf("A avatar: %d", rec.Code)
	}
	if rec := gated(b); strings.Contains(rec.Body.String(), "avatar_required") {
		t.Fatalf("B still blocked after A set the avatar: %d %s", rec.Code, rec.Body.String())
	}
	sess, err := srv.sessions.Load(context.Background(), b.cookies[auth.SessionCookie])
	if err != nil || !sess.ProfileComplete {
		t.Fatalf("B session not upgraded: %+v %v", sess, err)
	}
}

func TestRateLimit_RegisterOptionsAndLogin(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	got429 := false
	for i := 0; i < 15; i++ {
		if rec := do(t, srv, &jar{}, "GET", "/api/register/options?code=ZZZZZZZZZZZZ", nil, ""); rec.Code == 429 {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("register/options never returned 429")
	}
	got429 = false
	for i := 0; i < 15; i++ {
		if rec := do(t, srv, &jar{}, "POST", "/api/auth/login", dto.LoginRequest{Login: "nobody", Password: "whatever1"}, ""); rec.Code == 429 {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("login never returned 429")
	}
}
