// Package assessments: employee read side (visible list, take view) lives here;
// the manager side is in manage_repo.go / create_repo.go.
package assessments

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/httpapi/dto"
)

var (
	// ErrNotVisible: the assessment does not exist, is hidden, expired or archived (404).
	ErrNotVisible = errors.New("assessment not visible")
	// ErrAlreadyCompleted: the user already has an attempt for it (409).
	ErrAlreadyCompleted = errors.New("assessment already completed")
)

// VisibleSQL is the employee visibility predicate for an assessments alias `a` (plan sec. 4).
const VisibleSQL = `a.deleted_at IS NULL AND NOT a.hidden AND (a.expires_at IS NULL OR a.expires_at > now())`

// firstVisibleSQL is the id of the first test: the oldest employee-visible assessment.
var firstVisibleSQL = `(SELECT f.id FROM assessments f WHERE ` + strings.ReplaceAll(VisibleSQL, "a.", "f.") + ` ORDER BY f.created_at, f.id LIMIT 1)`

// GateSQL is the first-test rule for assessments alias `a`; `userParam` is the placeholder holding
// the user id (e.g. "$1"). Until a user completes any test, only the first test is reachable
// (list, take and submit); afterwards everything visible is.
func GateSQL(userParam string) string {
	return `(EXISTS (SELECT 1 FROM attempts gt WHERE gt.user_id = ` + userParam + `) OR a.id = ` + firstVisibleSQL + `)`
}

// EmployeeRepo is the employee-facing read side.
type EmployeeRepo struct{ pool *pgxpool.Pool }

func NewEmployeeRepo(pool *pgxpool.Pool) *EmployeeRepo { return &EmployeeRepo{pool: pool} }

// ListVisible returns the assessments visible to employees that the user has not completed yet;
// completed ones live in the history (profile timeline).
func (r *EmployeeRepo) ListVisible(ctx context.Context, userID int64) ([]dto.AssessmentListItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.title, a.description, a.expires_at,
		       (SELECT count(*) FROM questions q WHERE q.assessment_id = a.id), t.id
		  FROM assessments a
		  LEFT JOIN attempts t ON t.assessment_id = a.id AND t.user_id = $1
		 WHERE t.id IS NULL AND `+VisibleSQL+` AND `+GateSQL("$1")+`
		 ORDER BY a.created_at DESC, a.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dto.AssessmentListItem{}
	for rows.Next() {
		var it dto.AssessmentListItem
		if err := rows.Scan(&it.ID, &it.Title, &it.Description, &it.ExpiresAt, &it.QuestionCount, &it.AttemptID); err != nil {
			return nil, err
		}
		it.Completed = it.AttemptID != nil
		out = append(out, it)
	}
	return out, rows.Err()
}

// CountAvailable is the number of visible assessments the user has not completed yet.
func (r *EmployeeRepo) CountAvailable(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM assessments a
		 WHERE `+VisibleSQL+` AND `+GateSQL("$1")+`
		   AND NOT EXISTS (SELECT 1 FROM attempts t WHERE t.assessment_id = a.id AND t.user_id = $1)`, userID).Scan(&n)
	return n, err
}

// Take returns the question list WITHOUT effects. ErrNotVisible if hidden/expired/archived/missing,
// ErrAlreadyCompleted if the user already submitted it.
func (r *EmployeeRepo) Take(ctx context.Context, userID, id int64) (dto.TakeView, error) {
	var v dto.TakeView
	err := r.pool.QueryRow(ctx, `SELECT a.id, a.title, a.description FROM assessments a WHERE a.id = $1 AND `+VisibleSQL+` AND `+GateSQL("$2"), id, userID).
		Scan(&v.ID, &v.Title, &v.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotVisible
	}
	if err != nil {
		return v, err
	}
	var done bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM attempts WHERE user_id = $1 AND assessment_id = $2)`, userID, id).Scan(&done); err != nil {
		return v, err
	}
	if done {
		return v, ErrAlreadyCompleted
	}

	v.Questions = []dto.TakeQuestion{}
	byID := map[int64]int{}
	qrows, err := r.pool.Query(ctx, `SELECT id, position, text, type FROM questions WHERE assessment_id = $1 ORDER BY position`, id)
	if err != nil {
		return v, err
	}
	for qrows.Next() {
		var q dto.TakeQuestion
		if err := qrows.Scan(&q.ID, &q.Position, &q.Text, &q.Type); err != nil {
			qrows.Close()
			return v, err
		}
		q.Options = []dto.TakeOption{}
		byID[q.ID] = len(v.Questions)
		v.Questions = append(v.Questions, q)
	}
	qrows.Close()
	if err := qrows.Err(); err != nil {
		return v, err
	}
	orows, err := r.pool.Query(ctx, `
		SELECT o.question_id, o.id, o.position, o.text
		  FROM options o JOIN questions q ON q.id = o.question_id
		 WHERE q.assessment_id = $1 ORDER BY o.question_id, o.position`, id)
	if err != nil {
		return v, err
	}
	defer orows.Close()
	for orows.Next() {
		var qid int64
		var o dto.TakeOption
		if err := orows.Scan(&qid, &o.ID, &o.Position, &o.Text); err != nil {
			return v, err
		}
		i := byID[qid]
		v.Questions[i].Options = append(v.Questions[i].Options, o)
	}
	return v, orows.Err()
}
