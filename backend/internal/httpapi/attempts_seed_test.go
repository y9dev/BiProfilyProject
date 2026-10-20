package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/model"
)

// Seeding for the employee-side tests goes through plain SQL (no dependency on the
// manager create endpoint).

type seedEffect struct {
	char  int64
	dir   int
	level string
}

type seedOption struct {
	text    string
	effects []seedEffect
}

type seedQuestion struct {
	typ     string
	options []seedOption
}

type seeded struct {
	ID        int64
	Questions []int64
	Options   [][]int64 // [question][option]
}

func empChars(t *testing.T, pool *pgxpool.Pool) []int64 {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id FROM characteristics ORDER BY id`)
	if err != nil {
		t.Fatalf("characteristics: %v", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func seedAssessment(t *testing.T, pool *pgxpool.Pool, createdBy int64, title string, qs []seedQuestion) seeded {
	t.Helper()
	ctx := context.Background()
	var s seeded
	if err := pool.QueryRow(ctx, `INSERT INTO assessments (title, created_by) VALUES ($1, $2) RETURNING id`, title, createdBy).Scan(&s.ID); err != nil {
		t.Fatalf("seed assessment: %v", err)
	}
	for qi, q := range qs {
		var qid int64
		if err := pool.QueryRow(ctx, `INSERT INTO questions (assessment_id, position, text, type) VALUES ($1,$2,$3,$4) RETURNING id`,
			s.ID, qi, "Q"+itoa(int64(qi)), q.typ).Scan(&qid); err != nil {
			t.Fatalf("seed question: %v", err)
		}
		s.Questions = append(s.Questions, qid)
		var oids []int64
		for oi, o := range q.options {
			var oid int64
			if err := pool.QueryRow(ctx, `INSERT INTO options (question_id, position, text) VALUES ($1,$2,$3) RETURNING id`, qid, oi, o.text).Scan(&oid); err != nil {
				t.Fatalf("seed option: %v", err)
			}
			for _, e := range o.effects {
				if _, err := pool.Exec(ctx, `INSERT INTO option_effects (option_id, characteristic_id, level, direction) VALUES ($1,$2,$3,$4)`,
					oid, e.char, e.level, e.dir); err != nil {
					t.Fatalf("seed effect: %v", err)
				}
			}
			oids = append(oids, oid)
		}
		s.Options = append(s.Options, oids)
	}
	return s
}

// standardAssessment: Q1 single (c1 +high / +low / none), Q2 multiple (c1 +moderate & c2 +high / c2 -low / c1 +low).
// MAX c1 = 3 + 3 = 6, MAX c2 = 3.
func standardAssessment(t *testing.T, pool *pgxpool.Pool, createdBy int64, title string) seeded {
	t.Helper()
	ch := empChars(t, pool)
	c1, c2 := ch[0], ch[1]
	return seedAssessment(t, pool, createdBy, title, []seedQuestion{
		{typ: "single", options: []seedOption{
			{text: "a", effects: []seedEffect{{c1, 1, "high"}}},
			{text: "b", effects: []seedEffect{{c1, 1, "low"}}},
			{text: "c"},
		}},
		{typ: "multiple", options: []seedOption{
			{text: "d", effects: []seedEffect{{c1, 1, "moderate"}, {c2, 1, "high"}}},
			{text: "e", effects: []seedEffect{{c2, -1, "low"}}},
			{text: "f", effects: []seedEffect{{c1, 1, "low"}}},
		}},
	})
}

// empUser creates an employee with an avatar and a ready session.
func empUser(t *testing.T, srv *Server, login string) (model.User, *jar) {
	t.Helper()
	u := createUser(t, srv, login, "secret-123", model.RoleEmployee)
	setAvatar(t, srv, u.ID)
	return u, signIn(t, srv, u)
}

func empCloneJar(j *jar) *jar {
	c := &jar{cookies: map[string]string{}}
	for k, v := range j.cookies {
		c.cookies[k] = v
	}
	return c
}

func submitBody(pairs ...[]int64) map[string]any {
	// pairs: {questionId, optionId...}
	answers := []map[string]any{}
	for _, p := range pairs {
		answers = append(answers, map[string]any{"questionId": p[0], "optionIds": p[1:]})
	}
	return map[string]any{"answers": answers}
}

func empDecode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func empCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func mustExecSQL(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
