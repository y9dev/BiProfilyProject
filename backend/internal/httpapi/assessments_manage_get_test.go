package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sort"
	"testing"

	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

func keysOf(m map[string]any) []string {
	k := make([]string, 0, len(m))
	for n := range m {
		k = append(k, n)
	}
	sort.Strings(k)
	return k
}

func TestManageGet(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := managerJar(t, srv, "mgrget", model.RoleManager)
	emp := managerJar(t, srv, "empget", model.RoleEmployee)

	req := goldenCreateRequest(t)
	rec := do(t, srv, mgr, "POST", "/api/manage/assessments", req, csrfOf(mgr))
	want(t, rec, http.StatusCreated)
	id := parse[dto.CreateAssessmentResult](t, rec).ID
	path := fmt.Sprintf("/api/manage/assessments/%d", id)

	rec = do(t, srv, mgr, "GET", path, nil, "")
	want(t, rec, http.StatusOK)
	got := parse[dto.ManageAssessment](t, rec)
	if got.ID != id || got.Title != req.Title || got.AttemptCount != 0 || len(got.Questions) != len(req.Questions) {
		t.Fatalf("header mismatch: %+v", got)
	}
	for qi, q := range req.Questions {
		gq := got.Questions[qi]
		if gq.Position != qi || gq.Text != q.Text || gq.Type != q.Type || len(gq.Options) != len(q.Options) {
			t.Fatalf("question %d mismatch: %+v", qi, gq)
		}
		for oi, o := range q.Options {
			go_ := gq.Options[oi]
			if go_.Position != oi || go_.Text != o.Text || len(go_.Effects) != len(o.Effects) {
				t.Fatalf("q%d o%d mismatch: %+v", qi, oi, go_)
			}
			wantE := map[dto.Effect]bool{}
			for _, e := range o.Effects {
				wantE[e] = true
			}
			for _, e := range go_.Effects {
				if !wantE[e] {
					t.Fatalf("q%d o%d unexpected effect %+v", qi, oi, e)
				}
			}
		}
	}

	// Key shape matches the golden file.
	raw, err := os.ReadFile("../../../contracts/golden/manage_assessment.json")
	if err != nil {
		t.Fatal(err)
	}
	var g, a map[string]any
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(keysOf(g), keysOf(a)) {
		t.Fatalf("top keys %v, want %v", keysOf(a), keysOf(g))
	}
	gq := g["questions"].([]any)[0].(map[string]any)
	aq := a["questions"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(keysOf(gq), keysOf(aq)) {
		t.Fatalf("question keys %v, want %v", keysOf(aq), keysOf(gq))
	}
	go0 := gq["options"].([]any)[0].(map[string]any)
	ao0 := aq["options"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(keysOf(go0), keysOf(ao0)) {
		t.Fatalf("option keys %v, want %v", keysOf(ao0), keysOf(go0))
	}
	ge := go0["effects"].([]any)[0].(map[string]any)
	ae := ao0["effects"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(keysOf(ge), keysOf(ae)) {
		t.Fatalf("effect keys %v, want %v", keysOf(ae), keysOf(ge))
	}

	want(t, do(t, srv, emp, "GET", path, nil, ""), http.StatusForbidden)
	want(t, do(t, srv, mgr, "GET", "/api/manage/assessments/999999", nil, ""), http.StatusNotFound)
	want(t, do(t, srv, mgr, "GET", "/api/manage/assessments/abc", nil, ""), http.StatusNotFound)
}
