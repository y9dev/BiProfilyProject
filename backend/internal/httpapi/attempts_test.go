// Attempt tests: once-only submit, scoring vs stored answers, validation, the
// hide-vs-submit race, IDOR on results and the profile aggregation.
package httpapi

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/scoring"
)

func standardAnswers(a seeded) map[string]any {
	// Q1 option b (c1 +1), Q2 options d+e (c1 +2, c2 +3-1=+2): c1 3/6 = 50, c2 2/3 = 66.67.
	return submitBody([]int64{a.Questions[0], a.Options[0][1]}, []int64{a.Questions[1], a.Options[1][0], a.Options[1][1]})
}

func TestSubmit_Once(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	u, j := empUser(t, srv, "emp")
	a := standardAssessment(t, pool, mgr.ID, "A")
	path := "/api/assessments/" + itoa(a.ID) + "/submit"

	rec := do(t, srv, j, "POST", path, standardAnswers(a), csrfOf(j))
	if rec.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", rec.Code, rec.Body.String())
	}
	res := empDecode[dto.SubmitResult](t, rec)
	if len(res.Scores) != 2 || res.Scores[0].Score != 50 || res.Scores[1].Score != 66.67 {
		t.Fatalf("scores: %+v", res.Scores)
	}
	if res.Scores[0].NameI18n == nil || res.Scores[0].NameI18n["kk"] == "" {
		t.Fatalf("default characteristic must carry nameI18n: %+v", res.Scores[0])
	}

	rec = do(t, srv, j, "POST", path, standardAnswers(a), csrfOf(j))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "already_completed") {
		t.Fatalf("second: %d %s", rec.Code, rec.Body.String())
	}
	if n := empCount(t, pool, `SELECT count(*) FROM attempts WHERE user_id=$1`, u.ID); n != 1 {
		t.Fatalf("attempts = %d", n)
	}
	if n := empCount(t, pool, `SELECT count(*) FROM audit_log WHERE action='attempt_duplicate'`); n != 1 {
		t.Fatalf("audit attempt_duplicate = %d", n)
	}
	if n := empCount(t, pool, `SELECT count(*) FROM audit_log WHERE action='attempt_submit'`); n != 1 {
		t.Fatalf("audit attempt_submit = %d", n)
	}
	if n := empCount(t, pool, `SELECT count(*) FROM attempt_answers WHERE attempt_id=$1`, res.AttemptID); n != 3 {
		t.Fatalf("answers rows = %d", n)
	}
}

func TestSubmit_ConcurrentDouble(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	u, j := empUser(t, srv, "emp")
	a := standardAssessment(t, pool, mgr.ID, "A")
	path := "/api/assessments/" + itoa(a.ID) + "/submit"

	const n = 8
	codes := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jj := empCloneJar(j)
			<-start
			codes[i] = do(t, srv, jj, "POST", path, standardAnswers(a), csrfOf(jj)).Code
		}()
	}
	close(start)
	wg.Wait()
	created, conflicts := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	if created != 1 || conflicts != n-1 {
		t.Fatalf("codes %v: want exactly one 201 and %d 409", codes, n-1)
	}
	if c := empCount(t, pool, `SELECT count(*) FROM attempts WHERE user_id=$1`, u.ID); c != 1 {
		t.Fatalf("attempts = %d", c)
	}
	if c := empCount(t, pool, `SELECT count(*) FROM attempt_answers`); c != 3 {
		t.Fatalf("answers = %d (loser transactions must leave nothing)", c)
	}
}

// TestSubmit_ScoresMatch: stored scores equal the scoring package recomputed from the stored attempt_answers.
func TestSubmit_ScoresMatch(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j := empUser(t, srv, "emp")
	a := standardAssessment(t, pool, mgr.ID, "A")

	rec := do(t, srv, j, "POST", "/api/assessments/"+itoa(a.ID)+"/submit", standardAnswers(a), csrfOf(j))
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body.String())
	}
	res := empDecode[dto.SubmitResult](t, rec)

	ctx := context.Background()
	var qs []scoring.Question
	for qi, qid := range a.Questions {
		var typ string
		if err := pool.QueryRow(ctx, `SELECT type FROM questions WHERE id=$1`, qid).Scan(&typ); err != nil {
			t.Fatal(err)
		}
		q := scoring.Question{ID: qid, Type: typ}
		for _, oid := range a.Options[qi] {
			o := scoring.Option{ID: oid, Effects: map[int64]int{}}
			rows, err := pool.Query(ctx, `SELECT characteristic_id, level, direction FROM option_effects WHERE option_id=$1`, oid)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var cid int64
				var level string
				var dir int
				if err := rows.Scan(&cid, &level, &dir); err != nil {
					t.Fatal(err)
				}
				o.Effects[cid] = scoring.Signed(dir, level)
			}
			rows.Close()
			q.Options = append(q.Options, o)
		}
		qs = append(qs, q)
	}
	answers := map[int64][]int64{}
	rows, err := pool.Query(ctx, `SELECT question_id, option_id FROM attempt_answers WHERE attempt_id=$1 ORDER BY option_id`, res.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var qid, oid int64
		if err := rows.Scan(&qid, &oid); err != nil {
			t.Fatal(err)
		}
		answers[qid] = append(answers[qid], oid)
	}
	rows.Close()

	want := scoring.Compute(qs, answers)
	if len(want) != len(res.Scores) {
		t.Fatalf("rows: %d vs %d", len(want), len(res.Scores))
	}
	for i, w := range want {
		if res.Scores[i].CharacteristicID != w.CharacteristicID || res.Scores[i].Score != w.Score {
			t.Errorf("score %d: stored %+v, recomputed %+v", i, res.Scores[i], w)
		}
	}
	if n := empCount(t, pool, `SELECT count(*) FROM attempt_scores WHERE attempt_id=$1`, res.AttemptID); n != len(want) {
		t.Errorf("attempt_scores rows = %d", n)
	}
}

func TestSubmit_UnmeasuredCharacteristicHasNoRow(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j := empUser(t, srv, "emp")
	c := empChars(t, pool)
	// c[0] is only ever negative (MAX 0, no row); c[1] measurable.
	a := seedAssessment(t, pool, mgr.ID, "neg", []seedQuestion{{typ: "single", options: []seedOption{
		{text: "x", effects: []seedEffect{{c[0], -1, "high"}, {c[1], 1, "low"}}},
		{text: "y"},
	}}})
	rec := do(t, srv, j, "POST", "/api/assessments/"+itoa(a.ID)+"/submit", submitBody([]int64{a.Questions[0], a.Options[0][0]}), csrfOf(j))
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body.String())
	}
	res := empDecode[dto.SubmitResult](t, rec)
	if len(res.Scores) != 1 || res.Scores[0].CharacteristicID != c[1] || res.Scores[0].Score != 100 {
		t.Fatalf("scores: %+v", res.Scores)
	}
}

func TestSubmit_Validation(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j := empUser(t, srv, "emp")
	a := standardAssessment(t, pool, mgr.ID, "A")
	other := standardAssessment(t, pool, mgr.ID, "Other")
	path := "/api/assessments/" + itoa(a.ID) + "/submit"
	q0, q1 := a.Questions[0], a.Questions[1]

	cases := []struct {
		name string
		body map[string]any
		code string
	}{
		{"missing question", submitBody([]int64{q0, a.Options[0][0]}), "missing_answer"},
		{"unknown question", submitBody([]int64{q0, a.Options[0][0]}, []int64{q1, a.Options[1][0]}, []int64{999999, 1}), "unknown_question"},
		{"duplicate question", submitBody([]int64{q0, a.Options[0][0]}, []int64{q0, a.Options[0][1]}, []int64{q1, a.Options[1][0]}), "duplicate_question"},
		{"multiple options on single", submitBody([]int64{q0, a.Options[0][0], a.Options[0][1]}, []int64{q1, a.Options[1][0]}), "single_choice_requires_one"},
		{"empty multiple", submitBody([]int64{q0, a.Options[0][0]}, []int64{q1}), "empty_answer"},
		{"foreign option", submitBody([]int64{q0, other.Options[0][0]}, []int64{q1, a.Options[1][0]}), "unknown_option"},
		{"option of another question", submitBody([]int64{q0, a.Options[1][0]}, []int64{q1, a.Options[1][0]}), "unknown_option"},
		{"duplicate option", submitBody([]int64{q0, a.Options[0][0]}, []int64{q1, a.Options[1][0], a.Options[1][0]}), "duplicate_option"},
	}
	for _, tc := range cases {
		rec := do(t, srv, j, "POST", path, tc.body, csrfOf(j))
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), tc.code) {
			t.Errorf("%s: %d %s, want 422 %s", tc.name, rec.Code, rec.Body.String(), tc.code)
		}
	}
	if n := empCount(t, pool, `SELECT count(*) FROM attempts`); n != 0 {
		t.Fatalf("invalid submits created %d attempts", n)
	}
	if rec := do(t, srv, j, "POST", path, standardAnswers(a), csrfOf(j)); rec.Code != http.StatusCreated {
		t.Fatalf("valid after invalid: %d", rec.Code)
	}
	if rec := do(t, srv, j, "POST", path, standardAnswers(a), ""); rec.Code != http.StatusForbidden {
		t.Fatalf("submit without csrf: %d, want 403", rec.Code)
	}
}

// Hide commits first (submit queued behind the UPDATE's lock): submit must see it hidden -> 404, no attempt.
func TestSubmit_RaceWithHide(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	ctx := context.Background()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j := empUser(t, srv, "emp")
	a := standardAssessment(t, pool, mgr.ID, "A")

	// Deterministic part: an uncommitted hide holds the row lock; submit blocks, then 404s after commit.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE assessments SET hidden = true WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() {
		jj := empCloneJar(j)
		done <- do(t, srv, jj, "POST", "/api/assessments/"+itoa(a.ID)+"/submit", standardAnswers(a), csrfOf(jj)).Code
	}()
	select {
	case code := <-done:
		t.Fatalf("submit returned %d while the hide was uncommitted: no serialization", code)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != http.StatusNotFound {
		t.Fatalf("submit after hide commit: %d, want 404", code)
	}
	if n := empCount(t, pool, `SELECT count(*) FROM attempts WHERE assessment_id=$1`, a.ID); n != 0 {
		t.Fatalf("attempt created after hide: %d", n)
	}

	// Stress part: hide and submit in parallel; the outcome must always be consistent.
	for i := 0; i < 15; i++ {
		b := standardAssessment(t, pool, mgr.ID, "B")
		u, jb := empUser(t, srv, "racer"+itoa(int64(i)))
		var code int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			code = do(t, srv, jb, "POST", "/api/assessments/"+itoa(b.ID)+"/submit", standardAnswers(b), csrfOf(jb)).Code
		}()
		go func() {
			defer wg.Done()
			if _, err := pool.Exec(ctx, `UPDATE assessments SET hidden = true WHERE id = $1`, b.ID); err != nil {
				t.Error(err)
			}
		}()
		wg.Wait()
		n := empCount(t, pool, `SELECT count(*) FROM attempts WHERE assessment_id=$1 AND user_id=$2`, b.ID, u.ID)
		if (code == http.StatusCreated) != (n == 1) || (code != http.StatusCreated && code != http.StatusNotFound) {
			t.Fatalf("iteration %d: status %d with %d attempts", i, code, n)
		}
	}
}

func TestAttemptResult_NoIDOR(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j1 := empUser(t, srv, "emp1")
	_, j2 := empUser(t, srv, "emp2")
	a := standardAssessment(t, pool, mgr.ID, "A")
	rec := do(t, srv, j1, "POST", "/api/assessments/"+itoa(a.ID)+"/submit", standardAnswers(a), csrfOf(j1))
	res := empDecode[dto.SubmitResult](t, rec)
	path := "/api/me/attempts/" + itoa(res.AttemptID)

	rec = do(t, srv, j1, "GET", path, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("owner: %d %s", rec.Code, rec.Body.String())
	}
	got := empDecode[dto.AttemptResult](t, rec)
	if got.AttemptID != res.AttemptID || got.AssessmentID != a.ID || got.AssessmentTitle != "A" || len(got.Scores) != 2 {
		t.Fatalf("result: %+v", got)
	}
	if rec := do(t, srv, j2, "GET", path, nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("other user: %d, want 404", rec.Code)
	}
	if rec := do(t, srv, j1, "GET", "/api/me/attempts/999999", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing: %d, want 404", rec.Code)
	}
	if rec := do(t, srv, j1, "GET", "/api/me/attempts/abc", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("bad id: %d, want 404", rec.Code)
	}
}

func TestProfile_MeanAndTimeline(t *testing.T) {
	srv, pool, _ := testDeps(t)
	defer pool.Close()
	mgr := createUser(t, srv, "mgr", "secret-123", model.RoleManager)
	_, j := empUser(t, srv, "emp")
	_, jOther := empUser(t, srv, "other")
	c := empChars(t, pool)

	// A: c1 measurable (+high / none). B: c1 and c2. C: only c2; will be archived after the attempt.
	a := seedAssessment(t, pool, mgr.ID, "Alpha", []seedQuestion{{typ: "single", options: []seedOption{
		{text: "x", effects: []seedEffect{{c[0], 1, "high"}}}, {text: "y"}}}})
	b := seedAssessment(t, pool, mgr.ID, "Beta", []seedQuestion{{typ: "single", options: []seedOption{
		{text: "x", effects: []seedEffect{{c[0], 1, "high"}, {c[1], 1, "low"}}},
		{text: "y", effects: []seedEffect{{c[0], 1, "moderate"}, {c[1], 1, "low"}}}}}})
	cc := seedAssessment(t, pool, mgr.ID, "Gamma", []seedQuestion{{typ: "single", options: []seedOption{
		{text: "x", effects: []seedEffect{{c[1], 1, "high"}}}, {text: "y"}}}})
	spare := standardAssessment(t, pool, mgr.ID, "Spare")
	_ = spare

	submit := func(jj *jar, s seeded, opt int) dto.SubmitResult {
		rec := do(t, srv, jj, "POST", "/api/assessments/"+itoa(s.ID)+"/submit", submitBody([]int64{s.Questions[0], s.Options[0][opt]}), csrfOf(jj))
		if rec.Code != http.StatusCreated {
			t.Fatalf("submit: %d %s", rec.Code, rec.Body.String())
		}
		return empDecode[dto.SubmitResult](t, rec)
	}
	ra := submit(j, a, 1)   // c1 0
	rb := submit(j, b, 1)   // c1 2/3 = 66.67, c2 100
	rcc := submit(j, cc, 0) // c2 100
	submit(jOther, a, 0)    // another user's data must not leak in

	// Force a known chronological order independent of insert timing; archive Gamma afterwards.
	mustExecSQL(t, pool, `UPDATE attempts SET completed_at = '2026-10-01T09:00:00Z' WHERE id = $1`, rcc.AttemptID)
	mustExecSQL(t, pool, `UPDATE attempts SET completed_at = '2026-10-05T09:00:00Z' WHERE id = $1`, ra.AttemptID)
	mustExecSQL(t, pool, `UPDATE attempts SET completed_at = '2026-10-09T09:00:00Z' WHERE id = $1`, rb.AttemptID)
	mustExecSQL(t, pool, `UPDATE assessments SET deleted_at = now() WHERE id = $1`, cc.ID)

	rec := do(t, srv, j, "GET", "/api/me/profile", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("profile: %d %s", rec.Code, rec.Body.String())
	}
	p := empDecode[dto.Profile](t, rec)

	order := []int64{rcc.AttemptID, ra.AttemptID, rb.AttemptID}
	var got []int64
	for _, e := range p.Timeline {
		got = append(got, e.AttemptID)
	}
	if !slices.Equal(got, order) {
		t.Fatalf("timeline order %v, want %v", got, order)
	}
	if p.Timeline[0].AssessmentTitle != "Gamma" || len(p.Timeline[0].Scores) != 1 {
		t.Fatalf("archived assessment must stay in the timeline: %+v", p.Timeline[0])
	}
	if p.Stats.Completed != 3 {
		t.Fatalf("completed = %d", p.Stats.Completed)
	}
	// Available: Alpha, Beta done; Gamma archived; Spare visible and untouched.
	if p.Stats.Available != 1 {
		t.Fatalf("available = %d, want 1 (Spare)", p.Stats.Available)
	}
	// c1: Alpha 0, Beta 66.67 -> mean 33.34 (rounded) over 2 attempts; c2: Beta 100, Gamma 100 -> 100 over 2.
	// The profile always lists the default characteristics (count 0 when unmeasured).
	byID := map[int64]dto.ProfileCharacteristic{}
	for _, ch := range p.Characteristics {
		byID[ch.ID] = ch
	}
	c1, c2 := byID[c[0]], byID[c[1]]
	if c1.Count != 2 || c1.Mean != scoring.Mean([]float64{0, 66.67}) {
		t.Errorf("c1: %+v", c1)
	}
	if c2.Count != 2 || c2.Mean != 100 {
		t.Errorf("c2: %+v", c2)
	}
	if c1.NameI18n == nil {
		t.Errorf("default characteristic needs nameI18n: %+v", c1)
	}

	// A fresh user has an empty (non-null) timeline.
	_, jNew := empUser(t, srv, "fresh")
	raw := do(t, srv, jNew, "GET", "/api/me/profile", nil, "").Body.String()
	if !strings.Contains(raw, `"timeline":[]`) {
		t.Errorf("empty profile must serialize arrays: %s", raw)
	}
}
