package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/testutil"
)

// Shared helpers for the 5B manager/admin integration tests.

func signedIn(t *testing.T, srv *Server, pool *pgxpool.Pool, login, role string) (*jar, model.User) {
	t.Helper()
	u := testutil.User(t, pool, login, "secret-123", role, true)
	return signIn(t, srv, u), u
}

func call(t *testing.T, srv *Server, j *jar, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, srv, j, method, path, body, csrfOf(j))
}

func want(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status %d, want %d: %s", rec.Code, code, rec.Body.String())
	}
}

func parse[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("parse %q: %v", rec.Body.String(), err)
	}
	return v
}

func errCode(rec *httptest.ResponseRecorder) string {
	var e dto.Error
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	return e.Error
}

func future(d time.Duration) string { return time.Now().Add(d).UTC().Format(time.RFC3339) }

// ---------- invitations ----------

func invByCode(t *testing.T, srv *Server, j *jar, id int64) dto.Invitation {
	t.Helper()
	rec := do(t, srv, j, "GET", "/api/invitations", nil, "")
	want(t, rec, 200)
	for _, inv := range parse[[]dto.Invitation](t, rec) {
		if inv.ID == id {
			return inv
		}
	}
	t.Fatalf("invitation %d not in list", id)
	return dto.Invitation{}
}

func TestInvitations_CreateAndList(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, mgr := signedIn(t, srv, pool, "mgr", model.RoleManager)

	rec := call(t, srv, j, "POST", "/api/invitations", map[string]any{"expiresAt": future(48 * time.Hour), "maxRegistrations": 3})
	want(t, rec, 201)
	inv := parse[dto.Invitation](t, rec)
	if len(inv.Code) != 12 || inv.Status != "active" || inv.MaxRegistrations != 3 || inv.CreatedBy.ID != mgr.ID ||
		!strings.HasSuffix(inv.URL, "/register?code="+inv.Code) {
		t.Fatalf("created: %+v", inv)
	}
	// never expires
	rec = call(t, srv, j, "POST", "/api/invitations", map[string]any{"expiresAt": nil, "maxRegistrations": 1})
	want(t, rec, 201)
	if parse[dto.Invitation](t, rec).ExpiresAt != nil {
		t.Fatal("expiresAt should be null")
	}
	for _, bad := range []map[string]any{
		{"maxRegistrations": 0}, {"maxRegistrations": 10001},
		{"maxRegistrations": 2, "expiresAt": future(-time.Hour)},
	} {
		want(t, call(t, srv, j, "POST", "/api/invitations", bad), 422)
	}
	rec = do(t, srv, j, "GET", "/api/invitations", nil, "")
	want(t, rec, 200)
	if n := len(parse[[]dto.Invitation](t, rec)); n != 2 {
		t.Fatalf("list = %d, want 2", n)
	}
}

func TestInvitations_LowerMaxReactivateRevoke(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, mgr := signedIn(t, srv, pool, "mgr", model.RoleManager)
	id := testutil.Invitation(t, pool, mgr.ID, "AAAAAAAAAAAA", testutil.InviteOpts{Max: 5, Count: 3})
	path := fmt.Sprintf("/api/invitations/%d", id)

	rec := call(t, srv, j, "PATCH", path, map[string]any{"maxRegistrations": 2})
	want(t, rec, 409)
	if errCode(rec) != "max_below_count" {
		t.Fatalf("code %s", errCode(rec))
	}
	rec = call(t, srv, j, "PATCH", path, map[string]any{"maxRegistrations": 3})
	want(t, rec, 200)
	if st := parse[dto.Invitation](t, rec).Status; st != "full" {
		t.Fatalf("status %s, want full", st)
	}
	rec = call(t, srv, j, "PATCH", path, map[string]any{"maxRegistrations": 10})
	want(t, rec, 200)
	if st := parse[dto.Invitation](t, rec).Status; st != "active" {
		t.Fatalf("status %s, want active", st)
	}

	// expire, then reactivate with a future date; a past date is 422
	if _, err := pool.Exec(context.Background(), `UPDATE invitations SET expires_at = now() - interval '1 hour' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if st := invByCode(t, srv, j, id).Status; st != "expired" {
		t.Fatalf("status %s, want expired", st)
	}
	want(t, call(t, srv, j, "PATCH", path, map[string]any{"expiresAt": future(-time.Minute)}), 422)
	rec = call(t, srv, j, "PATCH", path, map[string]any{"expiresAt": future(24 * time.Hour)})
	want(t, rec, 200)
	if st := parse[dto.Invitation](t, rec).Status; st != "active" {
		t.Fatalf("status %s, want active (reactivated)", st)
	}
	rec = call(t, srv, j, "PATCH", path, map[string]any{"expiresAt": nil}) // never
	want(t, rec, 200)
	if parse[dto.Invitation](t, rec).ExpiresAt != nil {
		t.Fatal("expiresAt should be null")
	}

	// revoke: status revoked, patch refused, registration refused
	rec = call(t, srv, j, "POST", path+"/revoke", nil)
	want(t, rec, 200)
	if st := parse[dto.Invitation](t, rec).Status; st != "revoked" {
		t.Fatalf("status %s, want revoked", st)
	}
	rec = call(t, srv, j, "PATCH", path, map[string]any{"maxRegistrations": 20})
	want(t, rec, 409)
	if errCode(rec) != "revoked" {
		t.Fatalf("code %s", errCode(rec))
	}
	want(t, do(t, srv, &jar{}, "POST", "/api/auth/register", regBody("AAAAAAAAAAAA", "late.user"), ""), 410)
	want(t, call(t, srv, j, "POST", "/api/invitations/99999/revoke", nil), 404)
}

func registerVia(srv *Server, code, login, ip string) int {
	raw, _ := json.Marshal(regBody(code, login))
	req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(string(raw)))
	req.RemoteAddr = ip + ":1234" // distinct peers: XFF is ignored without trusted proxies
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec.Code
}

func TestInvitations_DeleteRules(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, mgr := signedIn(t, srv, pool, "mgr", model.RoleManager)
	unused := testutil.Invitation(t, pool, mgr.ID, "UNUSEDUNUSED", testutil.InviteOpts{Max: 2})
	used := testutil.Invitation(t, pool, mgr.ID, "USEDUSEDUSED", testutil.InviteOpts{Max: 2})
	if c := registerVia(srv, "USEDUSEDUSED", "reg.one", "10.0.0.1"); c != 201 {
		t.Fatalf("register: %d", c)
	}

	rec := call(t, srv, j, "DELETE", fmt.Sprintf("/api/invitations/%d", used), nil)
	want(t, rec, 409)
	if errCode(rec) != "has_registrations" {
		t.Fatalf("code %s", errCode(rec))
	}
	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/invitations/%d", unused), nil), 204)
	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/invitations/%d", unused), nil), 404)
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM invitations`).Scan(&n)
	if n != 1 {
		t.Fatalf("invitations left = %d, want 1", n)
	}
	// used -> revoke only
	rec = call(t, srv, j, "POST", fmt.Sprintf("/api/invitations/%d/revoke", used), nil)
	want(t, rec, 200)
}

func TestInvitations_Registrants(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, mgr := signedIn(t, srv, pool, "mgr", model.RoleManager)
	id := testutil.Invitation(t, pool, mgr.ID, "REGISTRANTSXX", testutil.InviteOpts{Max: 5})
	for i := 0; i < 3; i++ {
		if c := registerVia(srv, "REGISTRANTSXX", fmt.Sprintf("reg.%d", i), fmt.Sprintf("10.0.1.%d", i)); c != 201 {
			t.Fatalf("register %d: %d", i, c)
		}
	}
	path := fmt.Sprintf("/api/invitations/%d/registrations", id)
	rec := do(t, srv, j, "GET", path, nil, "")
	want(t, rec, 200)
	all := parse[[]dto.Registrant](t, rec)
	if len(all) != 3 || all[0].FullName == "" || all[0].JobTitle != "Инженер" {
		t.Fatalf("registrants: %+v", all)
	}
	rec = do(t, srv, j, "GET", path+"?limit=2&offset=2", nil, "")
	want(t, rec, 200)
	if page := parse[[]dto.Registrant](t, rec); len(page) != 1 || rec.Header().Get("X-Total-Count") != "3" {
		t.Fatalf("page: %+v total=%s", page, rec.Header().Get("X-Total-Count"))
	}
	want(t, do(t, srv, j, "GET", "/api/invitations/99999/registrations", nil, ""), 404)
}

// TestInvitations_ConcurrentRegisterAtLimit: 20 parallel registrations against max=5
// admit exactly 5 users, count 5, and 15 get 410 (AC-1, through the HTTP layer).
func TestInvitations_ConcurrentRegisterAtLimit(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	_, mgr := signedIn(t, srv, pool, "mgr", model.RoleManager)
	id := testutil.Invitation(t, pool, mgr.ID, "CONCURRENTXXX", testutil.InviteOpts{Max: 5})

	codes := make(chan int, 20)
	for i := 0; i < 20; i++ {
		go func(i int) {
			codes <- registerVia(srv, "CONCURRENTXXX", fmt.Sprintf("conc.%d", i), fmt.Sprintf("10.1.0.%d", i))
		}(i)
	}
	got := map[int]int{}
	for i := 0; i < 20; i++ {
		got[<-codes]++
	}
	if got[201] != 5 || got[410] != 15 {
		t.Fatalf("results: %v, want 5x201 and 15x410", got)
	}
	var users int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE invitation_id=$1`, id).Scan(&users)
	if users != 5 || inviteCount(t, pool, id) != 5 {
		t.Fatalf("users=%d count=%d, want 5/5", users, inviteCount(t, pool, id))
	}
}

func TestManagerRoutes_EmployeeForbidden(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, _ := signedIn(t, srv, pool, "emp", model.RoleEmployee)
	for _, r := range srv.Routes() {
		if r.Public || !strings.HasPrefix(r.Path, "/api/invitations") && !strings.HasPrefix(r.Path, "/api/departments") &&
			!strings.HasPrefix(r.Path, "/api/characteristics") && !strings.HasPrefix(r.Path, "/api/manage/") &&
			!strings.HasPrefix(r.Path, "/api/admin/") {
			continue
		}
		rec := do(t, srv, j, r.Method, fillPath(r.Path), map[string]any{}, csrfOf(j))
		if rec.Code != 403 {
			t.Errorf("employee %s: %d, want 403", r.Pattern(), rec.Code)
		}
	}
	mj, _ := signedIn(t, srv, pool, "mgr", model.RoleManager)
	for _, r := range srv.Routes() {
		if strings.HasPrefix(r.Path, "/api/admin/") {
			if rec := do(t, srv, mj, r.Method, fillPath(r.Path), map[string]any{}, csrfOf(mj)); rec.Code != 403 {
				t.Errorf("manager %s: %d, want 403", r.Pattern(), rec.Code)
			}
		}
	}
}

// ---------- departments ----------

func TestDepartments_CRUDAndSoftDeactivate(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, mgr := signedIn(t, srv, pool, "mgr", model.RoleManager)

	rec := call(t, srv, j, "POST", "/api/departments", map[string]any{"name": "  Продажи "})
	want(t, rec, 201)
	d := parse[dto.Department](t, rec)
	if d.Name != "Продажи" || !d.Active || d.UserCount != 0 {
		t.Fatalf("created: %+v", d)
	}
	want(t, call(t, srv, j, "POST", "/api/departments", map[string]any{"name": "продажи"}), 409) // case-insensitive
	want(t, call(t, srv, j, "POST", "/api/departments", map[string]any{"name": "  "}), 422)
	rec = call(t, srv, j, "PUT", fmt.Sprintf("/api/departments/%d", d.ID), map[string]any{"name": "Отдел продаж"})
	want(t, rec, 200)
	other := parse[dto.Department](t, call(t, srv, j, "POST", "/api/departments", map[string]any{"name": "ИТ"}))
	want(t, call(t, srv, j, "PUT", fmt.Sprintf("/api/departments/%d", other.ID), map[string]any{"name": "отдел ПРОДАЖ"}), 409)
	want(t, call(t, srv, j, "PUT", "/api/departments/9999", map[string]any{"name": "X"}), 404)

	// unused: hard delete
	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/departments/%d", other.ID), nil), 204)

	// in use: soft deactivate; the user keeps the department
	inv := testutil.Invitation(t, pool, mgr.ID, "DEPTINVITE12", testutil.InviteOpts{Max: 3})
	_ = inv
	body := regBody("DEPTINVITE12", "dept.user")
	body.DepartmentID = &d.ID
	want(t, do(t, srv, &jar{}, "POST", "/api/auth/register", body, ""), 201)
	rec = call(t, srv, j, "DELETE", fmt.Sprintf("/api/departments/%d", d.ID), nil)
	want(t, rec, 200)
	if !parse[dto.DepartmentDeleteResult](t, rec).Deactivated {
		t.Fatal("expected deactivated:true")
	}
	var deptID *int64
	if err := pool.QueryRow(context.Background(), `SELECT department_id FROM users WHERE login='dept.user'`).Scan(&deptID); err != nil || deptID == nil || *deptID != d.ID {
		t.Fatalf("user lost its department: %v %v", deptID, err)
	}
	// hidden from the default list and from register options; visible with includeInactive
	rec = do(t, srv, j, "GET", "/api/departments", nil, "")
	want(t, rec, 200)
	if n := len(parse[[]dto.Department](t, rec)); n != 0 {
		t.Fatalf("active list = %d, want 0", n)
	}
	rec = do(t, srv, j, "GET", "/api/departments?includeInactive=1", nil, "")
	want(t, rec, 200)
	if l := parse[[]dto.Department](t, rec); len(l) != 1 || l[0].Active || l[0].UserCount != 1 {
		t.Fatalf("inactive list: %+v", l)
	}
	rec = do(t, srv, &jar{}, "GET", "/api/register/options?code=DEPTINVITE12", nil, "")
	want(t, rec, 200)
	if strings.Contains(rec.Body.String(), "Отдел продаж") {
		t.Fatalf("inactive department offered at registration: %s", rec.Body.String())
	}
	// activate; a clash with an active namesake is 409
	other2 := parse[dto.Department](t, call(t, srv, j, "POST", "/api/departments", map[string]any{"name": "Отдел продаж"}))
	want(t, call(t, srv, j, "POST", fmt.Sprintf("/api/departments/%d/activate", d.ID), nil), 409)
	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/departments/%d", other2.ID), nil), 204)
	rec = call(t, srv, j, "POST", fmt.Sprintf("/api/departments/%d/activate", d.ID), nil)
	want(t, rec, 200)
	if !parse[dto.Department](t, rec).Active {
		t.Fatal("not active")
	}
}

// ---------- characteristics ----------

func newCharAssessment(t *testing.T, srv *Server, j *jar, charID int64) int64 {
	t.Helper()
	req := dto.CreateAssessmentRequest{
		Title: "Оценка", Description: "d",
		Questions: []dto.CreateQuestionRequest{{
			Text: "Вопрос?", Type: "single",
			Options: []dto.CreateOptionRequest{
				{Text: "Да", Effects: []dto.Effect{{CharacteristicID: charID, Level: "high", Direction: 1}}},
				{Text: "Нет", Effects: []dto.Effect{}},
			},
		}},
	}
	rec := call(t, srv, j, "POST", "/api/manage/assessments", req)
	want(t, rec, 201)
	return parse[dto.CreateAssessmentResult](t, rec).ID
}

func TestCharacteristics_Guards(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, _ := signedIn(t, srv, pool, "mgr", model.RoleManager)

	rec := do(t, srv, j, "GET", "/api/characteristics", nil, "")
	want(t, rec, 200)
	list := parse[[]dto.Characteristic](t, rec)
	if len(list) != 4 || !list[0].IsDefault || list[0].NameI18n["kk"] == "" || list[0].InUse {
		t.Fatalf("defaults: %+v", list)
	}
	defID := list[0].ID
	for _, c := range []struct{ method, path string }{{"PUT", ""}, {"DELETE", ""}} {
		var body any
		if c.method == "PUT" {
			body = map[string]any{"name": "Новое имя"}
		}
		rec = call(t, srv, j, c.method, fmt.Sprintf("/api/characteristics/%d", defID), body)
		want(t, rec, 409)
		if errCode(rec) != "default_protected" {
			t.Fatalf("%s: code %s", c.method, errCode(rec))
		}
	}
	// a direct SQL delete of a default that is in use would also fail (FK); defaults stay
	rec = call(t, srv, j, "POST", "/api/characteristics", map[string]any{"name": "Лидерство"})
	want(t, rec, 201)
	custom := parse[dto.Characteristic](t, rec)
	if custom.IsDefault || custom.NameI18n != nil {
		t.Fatalf("custom: %+v", custom)
	}
	want(t, call(t, srv, j, "POST", "/api/characteristics", map[string]any{"name": "лидерство"}), 409)
	want(t, call(t, srv, j, "POST", "/api/characteristics", map[string]any{"name": ""}), 422)
	rec = call(t, srv, j, "PUT", fmt.Sprintf("/api/characteristics/%d", custom.ID), map[string]any{"name": "Лидерские качества"})
	want(t, rec, 200)

	// used by an assessment: in_use with the assessments listed; the FK is the backstop
	aID := newCharAssessment(t, srv, j, custom.ID)
	rec = call(t, srv, j, "DELETE", fmt.Sprintf("/api/characteristics/%d", custom.ID), nil)
	want(t, rec, 409)
	e := parse[dto.Error](t, rec)
	if e.Error != "in_use" || len(e.Errors) != 1 || e.Errors[0].Path != fmt.Sprintf("assessments[%d]", aID) || e.Message != "Оценка" {
		t.Fatalf("in_use body: %s", rec.Body.String())
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM characteristics WHERE id=$1`, custom.ID); err == nil {
		t.Fatal("FK should block a direct delete of a used characteristic")
	}
	rec = do(t, srv, j, "GET", "/api/characteristics", nil, "")
	for _, c := range parse[[]dto.Characteristic](t, rec) {
		if c.ID == custom.ID && !c.InUse {
			t.Fatal("inUse should be true")
		}
	}
	// unused custom: deletable
	free := parse[dto.Characteristic](t, call(t, srv, j, "POST", "/api/characteristics", map[string]any{"name": "Свободная"}))
	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/characteristics/%d", free.ID), nil), 204)
	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/characteristics/%d", free.ID), nil), 404)
}

// ---------- assessments management ----------

func TestManageAssessments_MetaVisibilityExpiry(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, _ := signedIn(t, srv, pool, "mgr", model.RoleManager)
	var charID int64
	_ = pool.QueryRow(context.Background(), `SELECT id FROM characteristics ORDER BY id LIMIT 1`).Scan(&charID)
	id := newCharAssessment(t, srv, j, charID)
	path := fmt.Sprintf("/api/manage/assessments/%d", id)

	rec := do(t, srv, j, "GET", "/api/manage/assessments", nil, "")
	want(t, rec, 200)
	l := parse[[]dto.ManageAssessmentListItem](t, rec)
	if len(l) != 1 || l[0].Status != "active" || l[0].QuestionCount != 1 || l[0].AttemptCount != 0 {
		t.Fatalf("list: %+v", l)
	}
	rec = call(t, srv, j, "PATCH", path, map[string]any{"title": "  Новое  ", "description": "d2"})
	want(t, rec, 200)
	if it := parse[dto.ManageAssessmentListItem](t, rec); it.Title != "Новое" || it.Description != "d2" {
		t.Fatalf("patched: %+v", it)
	}
	rec = call(t, srv, j, "PATCH", path, map[string]any{"description": "only"}) // title untouched
	want(t, rec, 200)
	if it := parse[dto.ManageAssessmentListItem](t, rec); it.Title != "Новое" {
		t.Fatalf("title changed: %+v", it)
	}
	want(t, call(t, srv, j, "PATCH", path, map[string]any{"title": " "}), 422)
	want(t, call(t, srv, j, "PATCH", "/api/manage/assessments/9999", map[string]any{"title": "x"}), 404)

	rec = call(t, srv, j, "PUT", path+"/visibility", map[string]any{"hidden": true})
	want(t, rec, 200)
	if st := parse[dto.ManageAssessmentListItem](t, rec).Status; st != "hidden" {
		t.Fatalf("status %s", st)
	}
	want(t, call(t, srv, j, "PUT", path+"/visibility", map[string]any{}), 422)
	want(t, call(t, srv, j, "PUT", path+"/visibility", map[string]any{"hidden": false}), 200)

	want(t, call(t, srv, j, "PUT", path+"/expiry", map[string]any{"expiresAt": future(-time.Hour)}), 422)
	want(t, call(t, srv, j, "PUT", path+"/expiry", map[string]any{"expiresAt": future(time.Hour)}), 200)
	if _, err := pool.Exec(context.Background(), `UPDATE assessments SET expires_at = now() - interval '1 minute' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	rec = do(t, srv, j, "GET", "/api/manage/assessments", nil, "")
	if st := parse[[]dto.ManageAssessmentListItem](t, rec)[0].Status; st != "expired" {
		t.Fatalf("status %s, want expired", st)
	}
	rec = call(t, srv, j, "PUT", path+"/expiry", map[string]any{"expiresAt": nil})
	want(t, rec, 200)
	if it := parse[dto.ManageAssessmentListItem](t, rec); it.ExpiresAt != nil || it.Status != "active" {
		t.Fatalf("never: %+v", it)
	}
}

func TestManageAssessments_DeleteHardVsArchive(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j, _ := signedIn(t, srv, pool, "mgr", model.RoleManager)
	var charID int64
	_ = pool.QueryRow(context.Background(), `SELECT id FROM characteristics ORDER BY id LIMIT 1`).Scan(&charID)
	hard := newCharAssessment(t, srv, j, charID)
	archived := newCharAssessment(t, srv, j, charID)
	emp := testutil.Employee(t, pool, "emp", "secret-123")
	if _, err := pool.Exec(context.Background(), `INSERT INTO attempts (user_id, assessment_id) VALUES ($1,$2)`, emp.ID, archived); err != nil {
		t.Fatal(err)
	}

	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/manage/assessments/%d", hard), nil), 204)
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM assessments WHERE id=$1`, hard).Scan(&n)
	if n != 0 {
		t.Fatal("assessment without attempts must be hard-deleted")
	}
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM questions WHERE assessment_id=$1`, hard).Scan(&n)
	if n != 0 {
		t.Fatal("questions must cascade")
	}

	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/manage/assessments/%d", archived), nil), 204)
	var deleted *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT deleted_at FROM assessments WHERE id=$1`, archived).Scan(&deleted); err != nil || deleted == nil {
		t.Fatalf("assessment with attempts must be archived: %v %v", deleted, err)
	}
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM attempts WHERE assessment_id=$1`, archived).Scan(&n)
	if n != 1 {
		t.Fatal("attempt must be kept")
	}
	rec := do(t, srv, j, "GET", "/api/manage/assessments", nil, "")
	if l := parse[[]dto.ManageAssessmentListItem](t, rec); len(l) != 0 {
		t.Fatalf("archived listed: %+v", l)
	}
	ej := signIn(t, srv, emp)
	want(t, do(t, srv, ej, "GET", fmt.Sprintf("/api/assessments/%d/take", archived), nil, ""), 404)
	// archived rows cannot be edited or deleted again
	want(t, call(t, srv, j, "DELETE", fmt.Sprintf("/api/manage/assessments/%d", archived), nil), 404)
	want(t, call(t, srv, j, "PATCH", fmt.Sprintf("/api/manage/assessments/%d", archived), map[string]any{"title": "x"}), 404)
}

// ---------- admin ----------

func TestAdmin_RoleChange(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	aj, adm := signedIn(t, srv, pool, "boss", model.RoleAdmin)
	mj, mgr := signedIn(t, srv, pool, "mgr", model.RoleManager)
	_, emp := signedIn(t, srv, pool, "emp", model.RoleEmployee)

	// manager cannot change roles
	want(t, call(t, srv, mj, "PUT", fmt.Sprintf("/api/admin/users/%d/role", emp.ID), map[string]any{"role": "manager"}), 403)
	// self change
	rec := call(t, srv, aj, "PUT", fmt.Sprintf("/api/admin/users/%d/role", adm.ID), map[string]any{"role": "employee"})
	want(t, rec, 409)
	if errCode(rec) != "self_change" {
		t.Fatalf("code %s", errCode(rec))
	}
	// invalid role (admin cannot be granted)
	want(t, call(t, srv, aj, "PUT", fmt.Sprintf("/api/admin/users/%d/role", emp.ID), map[string]any{"role": "admin"}), 422)
	want(t, call(t, srv, aj, "PUT", "/api/admin/users/99999/role", map[string]any{"role": "manager"}), 404)

	// demoting a manager revokes their sessions (stale-role bug)
	want(t, do(t, srv, mj, "GET", "/api/characteristics", nil, ""), 200)
	rec = call(t, srv, aj, "PUT", fmt.Sprintf("/api/admin/users/%d/role", mgr.ID), map[string]any{"role": "employee"})
	want(t, rec, 200)
	if parse[dto.AdminUser](t, rec).Role != "employee" {
		t.Fatalf("role: %s", rec.Body.String())
	}
	want(t, do(t, srv, mj, "GET", "/api/characteristics", nil, ""), 401)
	// promotion
	want(t, call(t, srv, aj, "PUT", fmt.Sprintf("/api/admin/users/%d/role", emp.ID), map[string]any{"role": "manager"}), 200)
}

func TestAdmin_LastAdminGuard(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	_, a1 := signedIn(t, srv, pool, "admin1", model.RoleAdmin)
	_, a2 := signedIn(t, srv, pool, "admin2", model.RoleAdmin)
	ctx := context.Background()
	// two admins: demoting one is allowed
	if _, err := srv.admin.ChangeRole(ctx, a2.ID, model.RoleManager); err != nil {
		t.Fatalf("demote with another admin present: %v", err)
	}
	// a1 is now the only active admin: demote and block are refused
	if _, err := srv.admin.ChangeRole(ctx, a1.ID, model.RoleEmployee); err == nil || err.Error() != "last_admin" {
		t.Fatalf("demote last admin: %v", err)
	}
	if _, err := srv.admin.SetBlocked(ctx, a1.ID, true); err == nil || err.Error() != "last_admin" {
		t.Fatalf("block last admin: %v", err)
	}
	var role string
	_ = pool.QueryRow(ctx, `SELECT role FROM users WHERE id=$1`, a1.ID).Scan(&role)
	if role != model.RoleAdmin {
		t.Fatalf("role changed to %s", role)
	}
}

func TestAdmin_BlockUnblock(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	aj, adm := signedIn(t, srv, pool, "boss", model.RoleAdmin)
	ej, emp := signedIn(t, srv, pool, "emp", model.RoleEmployee)
	want(t, do(t, srv, ej, "GET", "/api/assessments", nil, ""), 200)

	rec := call(t, srv, aj, "POST", fmt.Sprintf("/api/admin/users/%d/block", adm.ID), nil)
	want(t, rec, 409)
	if errCode(rec) != "self_block" {
		t.Fatalf("code %s", errCode(rec))
	}
	rec = call(t, srv, aj, "POST", fmt.Sprintf("/api/admin/users/%d/block", emp.ID), nil)
	want(t, rec, 200)
	if parse[dto.AdminUser](t, rec).Status != "blocked" {
		t.Fatal("not blocked")
	}
	want(t, do(t, srv, ej, "GET", "/api/assessments", nil, ""), 401) // sessions revoked
	want(t, do(t, srv, &jar{}, "POST", "/api/auth/login", dto.LoginRequest{Login: "emp", Password: "secret-123"}, ""), 403)
	rec = call(t, srv, aj, "POST", fmt.Sprintf("/api/admin/users/%d/unblock", emp.ID), nil)
	want(t, rec, 200)
	want(t, do(t, srv, &jar{}, "POST", "/api/auth/login", dto.LoginRequest{Login: "emp", Password: "secret-123"}, ""), 200)
	want(t, call(t, srv, aj, "POST", "/api/admin/users/99999/block", nil), 404)
}

func TestResetPassword_RevokesSessions(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	aj, _ := signedIn(t, srv, pool, "boss", model.RoleAdmin)
	ej, emp := signedIn(t, srv, pool, "emp", model.RoleEmployee)
	path := fmt.Sprintf("/api/admin/users/%d/reset-password", emp.ID)

	want(t, call(t, srv, aj, "POST", path, map[string]any{"password": "short"}), 422)
	want(t, call(t, srv, aj, "POST", "/api/admin/users/99999/reset-password", map[string]any{"password": "new-password-1"}), 404)
	want(t, do(t, srv, ej, "GET", "/api/assessments", nil, ""), 200)
	rec := call(t, srv, aj, "POST", path, map[string]any{"password": "new-password-1"})
	want(t, rec, 204)
	if strings.Contains(rec.Body.String(), "new-password") {
		t.Fatal("password echoed")
	}
	want(t, do(t, srv, ej, "GET", "/api/assessments", nil, ""), 401)
	want(t, do(t, srv, &jar{}, "POST", "/api/auth/login", dto.LoginRequest{Login: "emp", Password: "secret-123"}, ""), 401)
	want(t, do(t, srv, &jar{}, "POST", "/api/auth/login", dto.LoginRequest{Login: "emp", Password: "new-password-1"}, ""), 200)
}

func TestAdmin_UsersList(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	aj, _ := signedIn(t, srv, pool, "boss", model.RoleAdmin)
	signedIn(t, srv, pool, "mgr", model.RoleManager)
	signedIn(t, srv, pool, "emp", model.RoleEmployee)

	check := func(q string, wantN int) {
		t.Helper()
		rec := do(t, srv, aj, "GET", "/api/admin/users"+q, nil, "")
		want(t, rec, 200)
		if l := parse[dto.AdminUserList](t, rec); len(l.Items) != wantN || l.Total != wantN {
			t.Fatalf("%s: %d/%d, want %d", q, len(l.Items), l.Total, wantN)
		}
	}
	check("", 3)
	check("?role=manager", 1)
	check("?search=emp", 1)
	check("?status=blocked", 0)
	_ = http.StatusOK
}
