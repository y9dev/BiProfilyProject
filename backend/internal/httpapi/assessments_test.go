// Employee-side assessment tests (list, take view, visibility). Manager-side
// tests (create, manage) live in their own files.
package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

func TestTakeView_NoEffects(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j := empUser(t, srv, "emp")
	a := standardAssessment(t, pool, mgr.ID, "Standard")

	rec := do(t, srv, j, "GET", "/api/assessments/"+itoa(a.ID)+"/take", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, banned := range []string{"effect", "direction", "level", "characteristic", "score", "weight"} {
		if strings.Contains(strings.ToLower(body), banned) {
			t.Errorf("take view leaks %q: %s", banned, body)
		}
	}
	v := empDecode[dto.TakeView](t, rec)
	if len(v.Questions) != 2 || len(v.Questions[0].Options) != 3 || v.Questions[1].Type != "multiple" {
		t.Fatalf("shape: %+v", v)
	}
	if v.Questions[0].Options[0].ID != a.Options[0][0] || v.Questions[0].Position != 0 {
		t.Fatalf("ids/order: %+v", v.Questions[0])
	}
}

func TestTake_NotVisible404(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j := empUser(t, srv, "emp")

	hidden := standardAssessment(t, pool, mgr.ID, "hidden")
	expired := standardAssessment(t, pool, mgr.ID, "expired")
	archived := standardAssessment(t, pool, mgr.ID, "archived")
	future := standardAssessment(t, pool, mgr.ID, "future")
	mustExecSQL(t, pool, `UPDATE assessments SET hidden = true WHERE id = $1`, hidden.ID)
	mustExecSQL(t, pool, `UPDATE assessments SET expires_at = now() - interval '1 minute' WHERE id = $1`, expired.ID)
	mustExecSQL(t, pool, `UPDATE assessments SET deleted_at = now() WHERE id = $1`, archived.ID)
	mustExecSQL(t, pool, `UPDATE assessments SET expires_at = now() + interval '1 day' WHERE id = $1`, future.ID)

	for name, id := range map[string]int64{"hidden": hidden.ID, "expired": expired.ID, "archived": archived.ID, "missing": 99999} {
		if rec := do(t, srv, j, "GET", "/api/assessments/"+itoa(id)+"/take", nil, ""); rec.Code != http.StatusNotFound {
			t.Errorf("take %s: %d, want 404", name, rec.Code)
		}
		body := submitBody([]int64{1, 1})
		if rec := do(t, srv, j, "POST", "/api/assessments/"+itoa(id)+"/submit", body, csrfOf(j)); rec.Code != http.StatusNotFound {
			t.Errorf("submit %s: %d, want 404", name, rec.Code)
		}
	}
	if rec := do(t, srv, j, "GET", "/api/assessments/"+itoa(future.ID)+"/take", nil, ""); rec.Code != http.StatusOK {
		t.Errorf("future expiry must stay visible: %d", rec.Code)
	}

	rec := do(t, srv, j, "GET", "/api/assessments", nil, "")
	items := empDecode[[]dto.AssessmentListItem](t, rec)
	if len(items) != 1 || items[0].ID != future.ID || items[0].ExpiresAt == nil || items[0].QuestionCount != 2 {
		t.Fatalf("list must hold only the visible one: %+v", items)
	}
}

func TestList_CompletionState(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j1 := empUser(t, srv, "emp1")
	_, j2 := empUser(t, srv, "emp2")
	a := standardAssessment(t, pool, mgr.ID, "A")
	b := standardAssessment(t, pool, mgr.ID, "B")

	body := submitBody([]int64{a.Questions[0], a.Options[0][0]}, []int64{a.Questions[1], a.Options[1][0]})
	rec := do(t, srv, j1, "POST", "/api/assessments/"+itoa(a.ID)+"/submit", body, csrfOf(j1))
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body.String())
	}
	res := empDecode[dto.SubmitResult](t, rec)

	items := empDecode[[]dto.AssessmentListItem](t, do(t, srv, j1, "GET", "/api/assessments", nil, ""))
	// Completed tests leave the list (they live in the profile history); the next one opens up.
	if len(items) != 1 || items[0].ID != b.ID || items[0].Completed || items[0].AttemptID != nil {
		t.Fatalf("list must hold only B for emp1: %+v", items)
	}
	if res.AttemptID == 0 {
		t.Fatalf("submit result: %+v", res)
	}
	// Completion is per user.
	for _, it := range empDecode[[]dto.AssessmentListItem](t, do(t, srv, j2, "GET", "/api/assessments", nil, "")) {
		if it.Completed || it.AttemptID != nil {
			t.Errorf("emp2 must see nothing completed: %+v", it)
		}
	}
	// Completed assessment: take is 409.
	if rec := do(t, srv, j1, "GET", "/api/assessments/"+itoa(a.ID)+"/take", nil, ""); rec.Code != http.StatusConflict ||
		!strings.Contains(rec.Body.String(), "already_completed") {
		t.Errorf("take completed: %d %s", rec.Code, rec.Body.String())
	}
}
