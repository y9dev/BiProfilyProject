package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

func goldenCreateRequest(t *testing.T) dto.CreateAssessmentRequest {
	t.Helper()
	b, err := os.ReadFile("../../../contracts/golden/create_assessment_request.json")
	if err != nil {
		t.Fatal(err)
	}
	var req dto.CreateAssessmentRequest
	if err := json.Unmarshal(b, &req); err != nil {
		t.Fatal(err)
	}
	// Golden uses a fixed date; make it a valid future one and ids the seeded 1..4.
	req.ExpiresAt = nil
	return req
}

func managerJar(t *testing.T, srv *Server, login string, role string) *jar {
	t.Helper()
	u := createUser(t, srv, login, "secret-123", role)
	setAvatar(t, srv, u.ID)
	return signIn(t, srv, u)
}

func TestCreate_Validation(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j := managerJar(t, srv, "mgr1", model.RoleManager)

	bad := goldenCreateRequest(t)
	bad.Title = ""
	bad.Questions[0].Options[0].Effects[0].CharacteristicID = 9999
	rec := do(t, srv, j, "POST", "/api/manage/assessments", bad, csrfOf(j))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var e dto.Error
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if e.Error != "validation" {
		t.Fatalf("error = %q", e.Error)
	}
	want := map[string]string{
		"title": "required",
		"questions[0].options[0].effects[0].characteristicId": "unknown_characteristic",
	}
	for _, fe := range e.Errors {
		delete(want, fe.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing errors %v in %s", want, rec.Body)
	}
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM assessments`).Scan(&n)
	if n != 0 {
		t.Fatalf("nothing may be inserted on 422, got %d assessments", n)
	}

	// Unmeasurable: no option raises any characteristic.
	unm := goldenCreateRequest(t)
	for qi := range unm.Questions {
		for oi := range unm.Questions[qi].Options {
			unm.Questions[qi].Options[oi].Effects = []dto.Effect{}
		}
	}
	rec = do(t, srv, j, "POST", "/api/manage/assessments", unm, csrfOf(j))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "no_measurable_characteristic") {
		t.Fatalf("unmeasurable: %d %s", rec.Code, rec.Body)
	}

	// Malformed body.
	rec = do(t, srv, j, "POST", "/api/manage/assessments", "not-an-object", csrfOf(j))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed: %d", rec.Code)
	}
}

func TestCreate_RolesAndCSRF(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	req := goldenCreateRequest(t)

	emp := managerJar(t, srv, "emp1", model.RoleEmployee)
	if rec := do(t, srv, emp, "POST", "/api/manage/assessments", req, csrfOf(emp)); rec.Code != http.StatusForbidden {
		t.Fatalf("employee: %d", rec.Code)
	}
	mgr := managerJar(t, srv, "mgr2", model.RoleManager)
	if rec := do(t, srv, mgr, "POST", "/api/manage/assessments", req, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf: %d", rec.Code)
	}
	adm := managerJar(t, srv, "adm1", model.RoleAdmin)
	if rec := do(t, srv, adm, "POST", "/api/manage/assessments", req, csrfOf(adm)); rec.Code != http.StatusCreated {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body)
	}
}

// TestCreateThenTake: create through the API, then read the definition back from the
// DB exactly as a take/scoring step would (ordered questions and options, effects stored).
// The take endpoint belongs to Step 4B; this test verifies the stored definitions.
func TestCreateThenTake(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j := managerJar(t, srv, "mgr3", model.RoleManager)

	req := goldenCreateRequest(t)
	rec := do(t, srv, j, "POST", "/api/manage/assessments", req, csrfOf(j))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var res dto.CreateAssessmentResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || res.ID == 0 || res.Warnings == nil {
		t.Fatalf("result %s (%v)", rec.Body, err)
	}
	ctx := context.Background()

	var title string
	var hidden bool
	if err := pool.QueryRow(ctx, `SELECT title, hidden FROM assessments WHERE id=$1 AND deleted_at IS NULL`, res.ID).Scan(&title, &hidden); err != nil || title != req.Title || hidden {
		t.Fatalf("assessment row: %q %v %v", title, hidden, err)
	}

	rows, err := pool.Query(ctx, `
		SELECT q.position, q.type, o.position, o.text, COALESCE(count(e.option_id),0)
		  FROM questions q JOIN options o ON o.question_id=q.id
		  LEFT JOIN option_effects e ON e.option_id=o.id
		 WHERE q.assessment_id=$1
		 GROUP BY q.position, q.type, o.position, o.text
		 ORDER BY q.position, o.position`, res.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var qp, op, ne int
		var typ, text string
		if err := rows.Scan(&qp, &typ, &op, &text, &ne); err != nil {
			t.Fatal(err)
		}
		wq := req.Questions[qp]
		wo := wq.Options[op]
		if typ != wq.Type || text != wo.Text || ne != len(wo.Effects) {
			t.Fatalf("q%d o%d mismatch: %s %q %d", qp, op, typ, text, ne)
		}
		i++
	}
	if total := len(req.Questions[0].Options) + len(req.Questions[1].Options); i != total {
		t.Fatalf("read %d options, want %d", i, total)
	}

	var level string
	var dir int
	if err := pool.QueryRow(ctx, `SELECT e.level, e.direction FROM option_effects e
		JOIN options o ON o.id=e.option_id JOIN questions q ON q.id=o.question_id
		WHERE q.assessment_id=$1 AND q.position=0 AND o.position=0 AND e.characteristic_id=3`, res.ID).Scan(&level, &dir); err != nil || level != "low" || dir != -1 {
		t.Fatalf("effect: %q %d %v", level, dir, err)
	}

	// Structure is immutable (trigger) even for direct SQL.
	if _, err := pool.Exec(ctx, `UPDATE questions SET text='x' WHERE assessment_id=$1`, res.ID); err == nil {
		t.Fatal("UPDATE questions must be rejected by the immutability trigger")
	}
	if _, err := pool.Exec(ctx, `UPDATE options SET text='x' WHERE question_id IN (SELECT id FROM questions WHERE assessment_id=$1)`, res.ID); err == nil {
		t.Fatal("UPDATE options must be rejected by the immutability trigger")
	}
}

func TestCreate_WarningsAndAtomicity(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	j := managerJar(t, srv, "mgr4", model.RoleManager)

	// Every option positive on characteristic 1: 0% unreachable -> 201 with warning.
	req := dto.CreateAssessmentRequest{Title: "W", Questions: []dto.CreateQuestionRequest{{
		Text: "q", Type: "single", Options: []dto.CreateOptionRequest{
			{Text: "a", Effects: []dto.Effect{{CharacteristicID: 1, Level: "high", Direction: 1}}},
			{Text: "b", Effects: []dto.Effect{{CharacteristicID: 1, Level: "low", Direction: 1}}},
		}}}}
	rec := do(t, srv, j, "POST", "/api/manage/assessments", req, csrfOf(j))
	if rec.Code != http.StatusCreated {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var res dto.CreateAssessmentResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if len(res.Warnings) != 1 || res.Warnings[0] != (dto.Warning{Path: "questions", Code: "min_unreachable", CharacteristicID: 1}) {
		t.Fatalf("warnings %+v", res.Warnings)
	}
}
