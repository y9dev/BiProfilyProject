// Package attempts: the once-per-user submit transaction, attempt results and the profile.
package attempts

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/assessments"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/scoring"
)

var (
	ErrNotVisible       = assessments.ErrNotVisible
	ErrAlreadyCompleted = assessments.ErrAlreadyCompleted
	// ErrNotFound: attempt missing or not owned by the caller (no IDOR: same answer).
	ErrNotFound = errors.New("attempt not found")
)

// ValidationError carries 422 field errors for an invalid answer set.
type ValidationError struct{ Errors []dto.FieldError }

func (e *ValidationError) Error() string { return "invalid answers" }

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

type definition struct {
	questions []scoring.Question
	positions map[int64]int // question id -> position (for error paths)
	optionQ   map[int64]int64
}

// loadDefinition reads questions, options and signed effects of an assessment inside tx.
func loadDefinition(ctx context.Context, tx pgx.Tx, assessmentID int64) (*definition, error) {
	d := &definition{positions: map[int64]int{}, optionQ: map[int64]int64{}}
	idx := map[int64]int{}
	rows, err := tx.Query(ctx, `SELECT id, position, type FROM questions WHERE assessment_id = $1 ORDER BY position`, assessmentID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var q scoring.Question
		var pos int
		if err := rows.Scan(&q.ID, &pos, &q.Type); err != nil {
			rows.Close()
			return nil, err
		}
		idx[q.ID] = len(d.questions)
		d.positions[q.ID] = pos
		d.questions = append(d.questions, q)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	optIdx := map[int64][2]int{}
	rows, err = tx.Query(ctx, `
		SELECT o.question_id, o.id FROM options o JOIN questions q ON q.id = o.question_id
		 WHERE q.assessment_id = $1 ORDER BY o.question_id, o.position`, assessmentID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var qid, oid int64
		if err := rows.Scan(&qid, &oid); err != nil {
			rows.Close()
			return nil, err
		}
		qi := idx[qid]
		optIdx[oid] = [2]int{qi, len(d.questions[qi].Options)}
		d.questions[qi].Options = append(d.questions[qi].Options, scoring.Option{ID: oid, Effects: map[int64]int{}})
		d.optionQ[oid] = qid
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = tx.Query(ctx, `
		SELECT e.option_id, e.characteristic_id, e.level, e.direction
		  FROM option_effects e JOIN options o ON o.id = e.option_id JOIN questions q ON q.id = o.question_id
		 WHERE q.assessment_id = $1`, assessmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var oid, cid int64
		var level string
		var dir int
		if err := rows.Scan(&oid, &cid, &level, &dir); err != nil {
			return nil, err
		}
		p := optIdx[oid]
		d.questions[p[0]].Options[p[1]].Effects[cid] = scoring.Signed(dir, level)
	}
	return d, rows.Err()
}

// validate checks the answer set against the definition. Every question is required;
// single/rating take exactly one option, multiple one or more distinct options.
func (d *definition) validate(in []dto.SubmitAnswer) (map[int64][]int64, []dto.FieldError) {
	var errs []dto.FieldError
	byQ := map[int64]scoring.Question{}
	for _, q := range d.questions {
		byQ[q.ID] = q
	}
	got := map[int64][]int64{}
	for i, a := range in {
		path := fmt.Sprintf("answers[%d]", i)
		q, ok := byQ[a.QuestionID]
		if !ok {
			errs = append(errs, dto.FieldError{Path: path + ".questionId", Code: "unknown_question"})
			continue
		}
		if _, dup := got[a.QuestionID]; dup {
			errs = append(errs, dto.FieldError{Path: path + ".questionId", Code: "duplicate_question"})
			continue
		}
		got[a.QuestionID] = a.OptionIDs
		if len(a.OptionIDs) == 0 {
			errs = append(errs, dto.FieldError{Path: path + ".optionIds", Code: "empty_answer"})
			continue
		}
		if q.Type != scoring.TypeMultiple && len(a.OptionIDs) != 1 {
			errs = append(errs, dto.FieldError{Path: path + ".optionIds", Code: "single_choice_requires_one"})
			continue
		}
		seen := map[int64]bool{}
		for j, oid := range a.OptionIDs {
			p := fmt.Sprintf("%s.optionIds[%d]", path, j)
			if d.optionQ[oid] != q.ID {
				errs = append(errs, dto.FieldError{Path: p, Code: "unknown_option"})
			} else if seen[oid] {
				errs = append(errs, dto.FieldError{Path: p, Code: "duplicate_option"})
			}
			seen[oid] = true
		}
	}
	for _, q := range d.questions {
		if _, ok := got[q.ID]; !ok {
			errs = append(errs, dto.FieldError{Path: fmt.Sprintf("questions[%d]", d.positions[q.ID]), Code: "missing_answer"})
		}
	}
	return got, errs
}

// Submit runs the whole submit in one transaction (plan sec. 7): the assessment row is
// locked FOR SHARE so a concurrent hide/expire/archive UPDATE serializes with it.
func (r *Repo) Submit(ctx context.Context, userID, assessmentID int64, answers []dto.SubmitAnswer) (dto.SubmitResult, error) {
	var res dto.SubmitResult
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	var lockedID int64
	err = tx.QueryRow(ctx, `
		SELECT a.id FROM assessments a
		 WHERE a.id = $1 AND `+assessments.VisibleSQL+` AND `+assessments.GateSQL("$2")+`
		 FOR SHARE OF a`, assessmentID, userID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrNotVisible
	}
	if err != nil {
		return res, err
	}

	def, err := loadDefinition(ctx, tx, assessmentID)
	if err != nil {
		return res, err
	}
	chosen, verrs := def.validate(answers)
	if len(verrs) > 0 {
		return res, &ValidationError{Errors: verrs}
	}
	results := scoring.Compute(def.questions, chosen)

	err = tx.QueryRow(ctx, `
		INSERT INTO attempts (user_id, assessment_id) VALUES ($1, $2)
		ON CONFLICT (user_id, assessment_id) DO NOTHING
		RETURNING id`, userID, assessmentID).Scan(&res.AttemptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrAlreadyCompleted
	}
	if err != nil {
		return res, err
	}

	batch := &pgx.Batch{}
	for _, q := range def.questions {
		for _, oid := range chosen[q.ID] {
			batch.Queue(`INSERT INTO attempt_answers (attempt_id, question_id, option_id) VALUES ($1, $2, $3)`, res.AttemptID, q.ID, oid)
		}
	}
	for _, s := range results {
		batch.Queue(`INSERT INTO attempt_scores (attempt_id, characteristic_id, score) VALUES ($1, $2, $3)`, res.AttemptID, s.CharacteristicID, s.Score)
	}
	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return res, err
		}
	}
	if err := br.Close(); err != nil {
		return res, err
	}

	res.Scores, err = scoresOf(ctx, tx, res.AttemptID)
	if err != nil {
		return res, err
	}
	if err := tx.Commit(ctx); err != nil {
		return res, err
	}
	return res, nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// scoresOf reads an attempt's stored scores with characteristic names, ordered by characteristic id.
func scoresOf(ctx context.Context, q querier, attemptID int64) ([]dto.Score, error) {
	rows, err := q.Query(ctx, `
		SELECT c.id, c.name, c.name_i18n, s.score::float8
		  FROM attempt_scores s JOIN characteristics c ON c.id = s.characteristic_id
		 WHERE s.attempt_id = $1 ORDER BY c.id`, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dto.Score{}
	for rows.Next() {
		var s dto.Score
		if err := rows.Scan(&s.CharacteristicID, &s.Name, &s.NameI18n, &s.Score); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Result returns an attempt owned by userID; anyone else's attempt is ErrNotFound (no IDOR).
func (r *Repo) Result(ctx context.Context, userID, attemptID int64) (dto.AttemptResult, error) {
	var out dto.AttemptResult
	err := r.pool.QueryRow(ctx, `
		SELECT t.id, t.assessment_id, a.title, t.completed_at
		  FROM attempts t JOIN assessments a ON a.id = t.assessment_id
		 WHERE t.id = $1 AND t.user_id = $2`, attemptID, userID).
		Scan(&out.AttemptID, &out.AssessmentID, &out.AssessmentTitle, &out.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	out.Scores, err = scoresOf(ctx, r.pool, attemptID)
	return out, err
}

// Profile is the user's mean per characteristic over all completed attempts (archived
// assessments still count: results are history) plus the chronological timeline.
// Stats.Available is filled by the caller.
func (r *Repo) Profile(ctx context.Context, userID int64) (dto.Profile, error) {
	p := dto.Profile{Characteristics: []dto.ProfileCharacteristic{}, Timeline: []dto.TimelineEntry{}}

	rows, err := r.pool.Query(ctx, `
		SELECT t.id, a.title, t.completed_at FROM attempts t JOIN assessments a ON a.id = t.assessment_id
		 WHERE t.user_id = $1 ORDER BY t.completed_at, t.id`, userID)
	if err != nil {
		return p, err
	}
	tl := map[int64]int{}
	for rows.Next() {
		var e dto.TimelineEntry
		if err := rows.Scan(&e.AttemptID, &e.AssessmentTitle, &e.CompletedAt); err != nil {
			rows.Close()
			return p, err
		}
		e.Scores = []dto.Score{}
		tl[e.AttemptID] = len(p.Timeline)
		p.Timeline = append(p.Timeline, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return p, err
	}
	p.Stats.Completed = len(p.Timeline)

	srows, err := r.pool.Query(ctx, `
		SELECT s.attempt_id, c.id, c.name, c.name_i18n, s.score::float8
		  FROM attempt_scores s
		  JOIN attempts t ON t.id = s.attempt_id
		  JOIN characteristics c ON c.id = s.characteristic_id
		 WHERE t.user_id = $1 ORDER BY s.attempt_id, c.id`, userID)
	if err != nil {
		return p, err
	}
	defer srows.Close()
	type agg struct {
		ch     dto.ProfileCharacteristic
		scores []float64
	}
	byChar := map[int64]*agg{}
	for srows.Next() {
		var aid int64
		var s dto.Score
		if err := srows.Scan(&aid, &s.CharacteristicID, &s.Name, &s.NameI18n, &s.Score); err != nil {
			return p, err
		}
		i := tl[aid]
		p.Timeline[i].Scores = append(p.Timeline[i].Scores, s)
		a := byChar[s.CharacteristicID]
		if a == nil {
			a = &agg{ch: dto.ProfileCharacteristic{ID: s.CharacteristicID, Name: s.Name, NameI18n: s.NameI18n}}
			byChar[s.CharacteristicID] = a
		}
		a.scores = append(a.scores, s.Score)
	}
	if err := srows.Err(); err != nil {
		return p, err
	}
	// The default characteristics always appear (count 0 until measured) so the main page
	// can show them before the first test is completed.
	drows, err := r.pool.Query(ctx, `SELECT id, name, name_i18n FROM characteristics WHERE is_default ORDER BY id`)
	if err != nil {
		return p, err
	}
	defer drows.Close()
	for drows.Next() {
		var c dto.ProfileCharacteristic
		if err := drows.Scan(&c.ID, &c.Name, &c.NameI18n); err != nil {
			return p, err
		}
		if byChar[c.ID] == nil {
			byChar[c.ID] = &agg{ch: c}
		}
	}
	if err := drows.Err(); err != nil {
		return p, err
	}
	ids := make([]int64, 0, len(byChar))
	for id := range byChar {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		a := byChar[id]
		if len(a.scores) > 0 {
			a.ch.Mean = scoring.Mean(a.scores)
		}
		a.ch.Count = len(a.scores)
		p.Characteristics = append(p.Characteristics, a.ch)
	}
	return p, nil
}
