// Command demoseed writes a self-contained SQL file with presentation demo data
// (departments, employees, assessments, attempts) or the SQL that removes it.
//
// Every demo user has a login starting with "demo."; demo assessments are created by
// demo.admin and titled "[Демо] ..."; demo departments and characteristics carry the
// marker created_at below. The seed SQL first removes previous demo rows, so it is
// re-runnable. Scores are computed with internal/scoring exactly like a real submit.
//
//	go run ./cmd/demoseed -password '<pw>' -out demo-seed.sql
//	go run ./cmd/demoseed -cleanup -out demo-cleanup.sql
//	docker cp demo-seed.sql bigroup-postgres-1:/tmp/ && docker exec bigroup-postgres-1 psql -U postgres -d bigroup -v ON_ERROR_STOP=1 -f /tmp/demo-seed.sql
package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"time"
	_ "time/tzdata"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/avatar"
	"bigroup-backend/internal/scoring"
)

// marker is the created_at of demo departments and characteristics (cleanup fingerprint).
const marker = "2025-09-01 00:00:00+00"

var almaty *time.Location

func main() {
	out := flag.String("out", "", "output SQL file (default stdout)")
	cleanup := flag.Bool("cleanup", false, "emit only the SQL that removes demo rows")
	password := flag.String("password", os.Getenv("DEMO_PASSWORD"), "shared demo password (or DEMO_PASSWORD)")
	seed := flag.Uint64("seed", 2026, "random seed")
	flag.Parse()

	var err error
	if almaty, err = time.LoadLocation("Asia/Almaty"); err != nil {
		fail(err)
	}
	var b strings.Builder
	b.WriteString("SET client_encoding = 'UTF8';\nBEGIN;\n")
	b.WriteString(cleanupSQL())
	if !*cleanup {
		if len(*password) < 8 || len(*password) > 128 {
			fail(fmt.Errorf("-password (or DEMO_PASSWORD) must be 8..128 characters"))
		}
		hash, err := auth.HashPassword(*password)
		if err != nil {
			fail(err)
		}
		g := &gen{rng: rand.New(rand.NewPCG(*seed, 7)), now: time.Now().UTC(), b: &b}
		g.seed(hash)
	}
	b.WriteString("COMMIT;\n")

	if *out == "" {
		fmt.Print(b.String())
		return
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o600); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "demoseed:", err)
	os.Exit(1)
}

func q(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func sqlList(xs []string) string {
	qs := make([]string, len(xs))
	for i, x := range xs {
		qs[i] = q(x)
	}
	return strings.Join(qs, ", ")
}

func deptNames() []string {
	var out []string
	for _, d := range depts {
		out = append(out, d.name)
	}
	return out
}

func extraNames() []string {
	var out []string
	for _, c := range extraCodes {
		out = append(out, charNames[c])
	}
	return out
}

// cleanupSQL removes only demo rows (and rows that reference them and would block the delete:
// attempts on demo assessments, audit entries of demo actors, invitations made by demo users).
func cleanupSQL() string {
	return `
CREATE TEMP TABLE demo_old_users ON COMMIT DROP AS SELECT id FROM users WHERE login LIKE 'demo.%';
CREATE TEMP TABLE demo_old_assess ON COMMIT DROP AS SELECT id FROM assessments WHERE created_by IN (SELECT id FROM demo_old_users);
DELETE FROM attempts WHERE user_id IN (SELECT id FROM demo_old_users) OR assessment_id IN (SELECT id FROM demo_old_assess);
DELETE FROM assessments WHERE id IN (SELECT id FROM demo_old_assess);
DELETE FROM audit_log WHERE actor_id IN (SELECT id FROM demo_old_users);
DELETE FROM invitations i WHERE i.created_by IN (SELECT id FROM demo_old_users)
   AND NOT EXISTS (SELECT 1 FROM users u WHERE u.invitation_id = i.id AND u.id NOT IN (SELECT id FROM demo_old_users));
DELETE FROM users WHERE id IN (SELECT id FROM demo_old_users);
DELETE FROM characteristics c WHERE NOT c.is_default AND c.created_at = '` + marker + `' AND c.name IN (` + sqlList(extraNames()) + `)
   AND NOT EXISTS (SELECT 1 FROM option_effects e WHERE e.characteristic_id = c.id)
   AND NOT EXISTS (SELECT 1 FROM attempt_scores s WHERE s.characteristic_id = c.id);
DELETE FROM departments d WHERE d.created_at = '` + marker + `' AND d.name IN (` + sqlList(deptNames()) + `)
   AND NOT EXISTS (SELECT 1 FROM users u WHERE u.department_id = d.id);
DROP TABLE demo_old_users;
DROP TABLE demo_old_assess;
`
}

type gen struct {
	rng *rand.Rand
	now time.Time
	b   *strings.Builder

	created []time.Time          // assessment created_at, by index
	defs    [][]scoring.Question // scoring definitions with synthetic ids
	charOf  map[int64]string     // synthetic characteristic id -> code
	optKey  map[int64]string     // synthetic option id -> demo_ids key
	qKey    map[int64]string     // synthetic question id -> demo_ids key

	stats stats
}

type stats struct {
	attempts, answers, scores int
	byMonth                   map[string][]float64
	last                      map[string]map[string]float64 // login -> code -> last score
	drops                     map[string]bool
}

func (g *gen) w(format string, a ...any) { fmt.Fprintf(g.b, format, a...) }

func (g *gen) seed(hash string) {
	g.w("\nCREATE TEMP TABLE demo_ids (k TEXT PRIMARY KEY, id BIGINT NOT NULL) ON COMMIT DROP;\n")

	// Departments: reuse an active department of the same name if one already exists.
	for _, d := range depts {
		g.w("INSERT INTO departments (name, active, created_at) SELECT %s, %t, '%s' WHERE %t = FALSE OR NOT EXISTS (SELECT 1 FROM departments WHERE active AND lower(name) = lower(%s));\n",
			q(d.name), d.active, marker, d.active, q(d.name))
		g.w("INSERT INTO demo_ids SELECT %s, id FROM departments WHERE lower(name) = lower(%s) AND (created_at = '%s' OR active) ORDER BY (created_at = '%s') DESC, id LIMIT 1;\n",
			q("d:"+d.name), q(d.name), marker, marker)
	}

	// Characteristics: core ones exist; extra ones are created unless present already.
	for _, c := range coreCodes {
		g.w("INSERT INTO demo_ids SELECT %s, id FROM characteristics WHERE is_default AND name = %s;\n", q("c:"+c), q(charNames[c]))
	}
	for _, c := range extraCodes {
		n := charNames[c]
		g.w("INSERT INTO characteristics (name, name_i18n, is_default, created_at) SELECT %s, NULL, FALSE, '%s' WHERE NOT EXISTS (SELECT 1 FROM characteristics WHERE lower(name) = lower(%s));\n", q(n), marker, q(n))
		g.w("INSERT INTO demo_ids SELECT %s, id FROM characteristics WHERE lower(name) = lower(%s);\n", q("c:"+c), q(n))
	}
	g.w("DO $$ BEGIN IF (SELECT count(*) FROM demo_ids WHERE k LIKE 'c:%%') <> %d THEN RAISE EXCEPTION 'demo seed: missing characteristics'; END IF; END $$;\n", len(charNames))

	// Users.
	g.stats = stats{byMonth: map[string][]float64{}, last: map[string]map[string]float64{}, drops: map[string]bool{}}
	for _, a := range assessmentDefs {
		g.created = append(g.created, g.now.Add(-g.day(a.ageDays)).Truncate(time.Hour).Add(3*time.Hour+17*time.Minute))
	}
	plans := make([]plan, len(people))
	for i, p := range people {
		plans[i] = g.plan(p)
	}
	for i, p := range people {
		dep := "NULL"
		if p.dept != "" {
			dep = fmt.Sprintf("(SELECT id FROM demo_ids WHERE k = %s)", q("d:"+p.dept))
		}
		status := "active"
		if p.profile == pBlocked {
			status = "blocked"
		}
		created := plans[i].joined
		av := avatarFor(g.rng, p.gender)
		g.w("WITH u AS (INSERT INTO users (login, pass_hash, full_name, job_title, department_id, role, status, avatar_universe, avatar_gender, avatar_style, avatar_set_at, created_at)\n"+
			"  VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s) RETURNING id)\nINSERT INTO demo_ids SELECT %s, id FROM u;\n",
			q(p.login), q(hash), q(p.name), q(p.title), dep, q(p.role), q(status),
			q(av[0]), q(av[1]), q(av[2]), ts(created.Add(12*time.Minute)), ts(created), q("u:"+p.login))
	}

	g.assessments()

	// Attempts in chronological order.
	type ev struct {
		login string
		a     int
		at    time.Time
		pen   float64
		lat   latent
	}
	var evs []ev
	for i, p := range people {
		for _, at := range plans[i].attempts {
			evs = append(evs, ev{p.login, at.a, at.at, at.penalty, plans[i].lat})
		}
	}
	slices.SortFunc(evs, func(x, y ev) int { return x.at.Compare(y.at) })
	for _, e := range evs {
		g.attempt(e.login, e.a, e.at, e.lat, e.pen)
	}
	g.report()
}

// latent is a person's ability model: level per characteristic at the D1 date plus growth.
type latent struct {
	base  map[string]float64
	slope float64
}

type attemptPlan struct {
	a       int
	at      time.Time
	penalty float64
}

type plan struct {
	joined   time.Time
	attempts []attemptPlan
	lat      latent
}

func (g *gen) day(d int) time.Duration { return time.Duration(d) * 24 * time.Hour }

// atTime puts an instant on a working hour of the Almaty day containing t; ~10% land
// within minutes of Almaty midnight on either side.
func (g *gen) atTime(t time.Time) time.Time {
	l := t.In(almaty)
	d := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, almaty)
	switch r := g.rng.Float64(); {
	case r < 0.05:
		return d.Add(23*time.Hour + time.Duration(40+g.rng.IntN(19))*time.Minute)
	case r < 0.10:
		return d.Add(time.Duration(2+g.rng.IntN(25)) * time.Minute)
	default:
		return d.Add(9*time.Hour + time.Duration(g.rng.IntN(10*60))*time.Minute + time.Duration(g.rng.IntN(60))*time.Second)
	}
}

func (g *gen) plan(p person) plan {
	d1 := g.now.Add(-g.day(assessmentDefs[0].ageDays))
	var dp dept
	for _, d := range depts {
		if d.name == p.dept {
			dp = d
		}
	}
	if p.dept == "" {
		dp = dept{offset: 0, slope: 0.01}
	}
	lat := latent{base: map[string]float64{}, slope: dp.slope + g.rng.NormFloat64()*0.004}
	person := g.rng.NormFloat64() * 0.07
	for c := range charNames {
		lat.base[c] = 0.50 + dp.offset + person + g.rng.NormFloat64()*0.06
	}
	pl := plan{lat: lat}

	if p.profile == pNoTests {
		pl.joined = g.atTime(g.now.Add(-g.day(10 + g.rng.IntN(150))))
		return pl
	}
	// Join early enough for a history; staff accounts exist from the start.
	span := 200
	if p.profile == pOutdated {
		span = 120
	}
	join := d1.Add(g.day(1 + g.rng.IntN(span)))
	if p.role != "employee" {
		join = d1.Add(-g.day(20 + g.rng.IntN(10)))
	}
	pl.joined = g.atTime(join)

	// Last allowed instant per profile.
	limit := g.now.Add(-time.Hour)
	switch p.profile {
	case pOutdated:
		limit = g.now.Add(-g.day(35 + g.rng.IntN(60)))
	case pBlocked:
		limit = g.now.Add(-g.day(50 + g.rng.IntN(40)))
	}
	// A recent final test for active/drop employees (about half in the last week).
	var final time.Time
	if p.role == "employee" && (p.profile == pActive || p.profile == pDrop) {
		days := g.rng.IntN(28)
		if g.rng.Float64() < 0.45 {
			days = g.rng.IntN(7)
		}
		final = g.atTime(g.now.Add(-g.day(days)))
		if final.After(limit) {
			final = limit
		}
	}

	prev := pl.joined.Add(g.day(1 + g.rng.IntN(6)))
	if p.role != "employee" {
		prev = d1.Add(g.day(1 + g.rng.IntN(5)))
	}
	n := len(assessmentDefs)
	if p.role != "employee" {
		n = 2
	}
	for a := 0; a < n; a++ {
		created := g.created[a]
		at := prev
		if a > 0 {
			at = prev.Add(g.day(7 + g.rng.IntN(35)))
			if c := created.Add(g.day(1 + g.rng.IntN(45))); c.After(at) {
				at = c
			}
			if a < n-1 && g.rng.Float64() < 0.15 {
				continue // skipped this one
			}
		}
		at = g.atTime(at)
		if a == n-1 && !final.IsZero() {
			at = final
		}
		if !at.After(created) || !at.Before(limit) {
			break
		}
		if !final.IsZero() && a < n-1 && !at.Before(final.Add(-g.day(1))) {
			break
		}
		pen := 0.0
		if p.profile == pDrop && a == n-1 {
			pen = 0.2
		}
		pl.attempts = append(pl.attempts, attemptPlan{a, at, pen})
		prev = at
	}
	// Active/drop employees always end with the recent final test.
	if !final.IsZero() {
		if len(pl.attempts) == 0 || pl.attempts[len(pl.attempts)-1].a != n-1 {
			pen := 0.0
			if p.profile == pDrop {
				pen = 0.2
			}
			pl.attempts = append(pl.attempts, attemptPlan{n - 1, final, pen})
		}
	}
	if len(pl.attempts) == 0 { // every non-notests profile keeps at least the first test
		at := pl.joined.Add(g.day(2))
		if at.After(limit) {
			at = limit.Add(-g.day(1))
		}
		pl.attempts = []attemptPlan{{0, at, 0}}
		if pl.joined.After(at) {
			pl.joined = at.Add(-g.day(3))
		}
	}
	return pl
}

func avatarFor(rng *rand.Rand, gender string) [3]string {
	return [3]string{avatar.Universes[rng.IntN(len(avatar.Universes))], gender, avatar.Styles[rng.IntN(len(avatar.Styles))]}
}

func ts(t time.Time) string { return q(t.UTC().Format("2006-01-02 15:04:05.000000+00")) }

// assessments writes the definitions and builds the scoring models with synthetic ids.
func (g *gen) assessments() {
	g.charOf = map[int64]string{}
	g.optKey = map[int64]string{}
	g.qKey = map[int64]string{}
	charID := map[string]int64{}
	codes := append(slices.Clone(coreCodes), extraCodes...)
	for i, c := range codes {
		charID[c] = int64(i + 1)
		g.charOf[int64(i+1)] = c
	}
	var nextID int64 = 1000
	for ai, a := range assessmentDefs {
		created := g.created[ai]
		ak := fmt.Sprintf("a:%d", ai)
		g.w("\nWITH a AS (INSERT INTO assessments (title, description, hidden, expires_at, created_by, created_at, updated_at)\n"+
			"  VALUES (%s, %s, FALSE, NULL, (SELECT id FROM demo_ids WHERE k = 'u:demo.admin'), %s, %s) RETURNING id)\nINSERT INTO demo_ids SELECT %s, id FROM a;\n",
			q(a.title), q(a.desc), ts(created), ts(created), q(ak))
		var def []scoring.Question
		for qi, qs := range a.qs {
			nextID++
			sq := scoring.Question{ID: nextID, Type: qs.typ}
			qk := fmt.Sprintf("q:%d:%d", ai, qi)
			g.qKey[sq.ID] = qk
			g.w("WITH x AS (INSERT INTO questions (assessment_id, position, text, type) VALUES ((SELECT id FROM demo_ids WHERE k = %s), %d, %s, %s) RETURNING id) INSERT INTO demo_ids SELECT %s, id FROM x;\n",
				q(ak), qi, q(qs.text), q(qs.typ), q(qk))
			for oi, op := range qs.opts {
				nextID++
				so := scoring.Option{ID: nextID, Effects: map[int64]int{}}
				ok := fmt.Sprintf("o:%d:%d:%d", ai, qi, oi)
				g.optKey[so.ID] = ok
				g.w("WITH x AS (INSERT INTO options (question_id, position, text) VALUES ((SELECT id FROM demo_ids WHERE k = %s), %d, %s) RETURNING id) INSERT INTO demo_ids SELECT %s, id FROM x;\n",
					q(qk), oi, q(op.text), q(ok))
				for _, e := range op.effs {
					code, dir, level := parseEffect(e)
					if _, known := charID[code]; !known {
						fail(fmt.Errorf("unknown characteristic code in %q", e))
					}
					so.Effects[charID[code]] = scoring.Signed(dir, level)
					g.w("INSERT INTO option_effects (option_id, characteristic_id, level, direction) VALUES ((SELECT id FROM demo_ids WHERE k = %s), (SELECT id FROM demo_ids WHERE k = %s), %s, %d);\n",
						q(ok), q("c:"+code), q(level), dir)
				}
				sq.Options = append(sq.Options, so)
			}
			def = append(def, sq)
		}
		if bad := scoring.MinUnreachable(def); len(bad) > 0 {
			fmt.Fprintf(os.Stderr, "note: %s: 0%% unreachable for %v\n", a.title, bad)
		}
		g.defs = append(g.defs, def)
	}
}

func parseEffect(e string) (code string, dir int, level string) {
	i := strings.LastIndexAny(e, "+-")
	code, dir = e[:i], 1
	if e[i] == '-' {
		dir = -1
	}
	level = map[string]string{"l": "low", "m": "moderate", "h": "high"}[e[i+1:]]
	if level == "" || code == "" {
		fail(fmt.Errorf("bad effect %q", e))
	}
	return code, dir, level
}

// answer picks options that fit the latent levels theta (0..1 per characteristic):
// single/rating take the option whose relative position on each affected characteristic
// is closest to theta (plus a little jitter); multiple includes a positive option with
// probability theta and a negative one with probability 1-theta.
func (g *gen) answer(def []scoring.Question, theta map[string]float64) map[int64][]int64 {
	out := map[int64][]int64{}
	for _, qs := range def {
		if qs.Type == scoring.TypeMultiple {
			var picked []int64
			for _, op := range qs.Options {
				pos, n := 0.0, 0
				for c, e := range op.Effects {
					t := theta[g.charOf[c]]
					if e < 0 {
						t = (1 - t) * 0.25
					}
					pos += t
					n++
				}
				p := 0.25
				if n > 0 {
					p = pos / float64(n)
				}
				if g.rng.Float64() < p {
					picked = append(picked, op.ID)
				}
			}
			if len(picked) == 0 {
				picked = []int64{qs.Options[g.rng.IntN(len(qs.Options))].ID}
			}
			out[qs.ID] = picked
			continue
		}
		lo, hi := map[int64]int{}, map[int64]int{}
		for _, op := range qs.Options {
			for c := range op.Effects {
				lo[c], hi[c] = 0, 0
			}
		}
		for c := range lo {
			for i, op := range qs.Options {
				e := op.Effects[c]
				if i == 0 || e < lo[c] {
					lo[c] = e
				}
				if i == 0 || e > hi[c] {
					hi[c] = e
				}
			}
		}
		best, bestD := int64(0), math.Inf(1)
		for _, op := range qs.Options {
			d := 0.0
			for c := range lo {
				if hi[c] == lo[c] {
					continue
				}
				target := theta[g.charOf[c]] * float64(max(hi[c], 0))
				d += math.Abs(float64(op.Effects[c])-target) / float64(hi[c]-lo[c])
			}
			d += g.rng.NormFloat64() * 0.05
			if d < bestD {
				best, bestD = op.ID, d
			}
		}
		out[qs.ID] = []int64{best}
	}
	return out
}

func (g *gen) attempt(login string, a int, at time.Time, lat latent, penalty float64) {
	months := at.Sub(g.created[0]).Hours() / 24 / 30.4
	theta := map[string]float64{}
	for c, b := range lat.base {
		v := b + lat.slope*months + g.rng.NormFloat64()*0.03 - penalty
		theta[c] = math.Max(0.02, math.Min(0.98, v))
	}
	def := g.defs[a]
	// Resample the answer set until it matches the intended story: a clear drop (>= 15 on
	// some characteristic vs. its previous score) for a penalized attempt, none otherwise.
	var answers map[int64][]int64
	var results []scoring.Result
	for try := 0; try < 400; try++ {
		nudged := map[string]float64{}
		for c, v := range theta { // drift toward the story on repeated misses
			step := 0.005 * float64(try)
			if penalty > 0 {
				step = -step / 2
			}
			nudged[c] = math.Max(0.02, math.Min(0.98, v+step))
		}
		answers = g.answer(def, nudged)
		results = scoring.Compute(def, answers)
		dropped := false
		for _, r := range results {
			if prev, ok := g.stats.last[login][g.charOf[r.CharacteristicID]]; ok && r.Score-prev <= -15 {
				dropped = true
			}
		}
		if dropped == (penalty > 0) {
			break
		}
	}

	var av, sv []string
	for _, qs := range def {
		for _, oid := range answers[qs.ID] {
			av = append(av, fmt.Sprintf("(%s, %s)", q(g.qKey[qs.ID]), q(g.optKey[oid])))
		}
	}
	for _, r := range results {
		code := g.charOf[r.CharacteristicID]
		sv = append(sv, fmt.Sprintf("(%s, %.2f)", q("c:"+code), r.Score))
		if slices.Contains(coreCodes, code) {
			m := at.In(almaty).Format("2006-01")
			g.stats.byMonth[m] = append(g.stats.byMonth[m], r.Score)
		}
		if g.stats.last[login] == nil {
			g.stats.last[login] = map[string]float64{}
		}
		if prev, ok := g.stats.last[login][code]; ok {
			g.stats.drops[login+" "+code] = r.Score-prev <= -15 // only the last two measurements count
		}
		g.stats.last[login][code] = r.Score
	}
	g.stats.attempts++
	g.stats.answers += len(av)
	g.stats.scores += len(sv)
	g.w("WITH t AS (INSERT INTO attempts (user_id, assessment_id, completed_at) VALUES ((SELECT id FROM demo_ids WHERE k = %s), (SELECT id FROM demo_ids WHERE k = %s), %s) RETURNING id),\n"+
		"  aa AS (INSERT INTO attempt_answers (attempt_id, question_id, option_id) SELECT t.id, qi.id, oi.id FROM t, (VALUES %s) v(qk, ok) JOIN demo_ids qi ON qi.k = v.qk JOIN demo_ids oi ON oi.k = v.ok)\n"+
		"INSERT INTO attempt_scores (attempt_id, characteristic_id, score) SELECT t.id, ci.id, v.s FROM t, (VALUES %s) v(ck, s) JOIN demo_ids ci ON ci.k = v.ck;\n",
		q("u:"+login), q(fmt.Sprintf("a:%d", a)), ts(at), strings.Join(av, ", "), strings.Join(sv, ", "))
}

func (g *gen) report() {
	fmt.Fprintf(os.Stderr, "users=%d assessments=%d attempts=%d answers=%d scores=%d\n",
		len(people), len(assessmentDefs), g.stats.attempts, g.stats.answers, g.stats.scores)
	var months []string
	for m := range g.stats.byMonth {
		months = append(months, m)
	}
	slices.Sort(months)
	for _, m := range months {
		fmt.Fprintf(os.Stderr, "  %s core mean=%.1f (n=%d)\n", m, scoring.Mean(g.stats.byMonth[m]), len(g.stats.byMonth[m]))
	}
	var drops []string
	for l, d := range g.stats.drops {
		if d {
			drops = append(drops, l)
		}
	}
	slices.Sort(drops)
	fmt.Fprintf(os.Stderr, "latest-vs-previous drops >= 15 (user characteristic): %d %v\n", len(drops), drops)
}
