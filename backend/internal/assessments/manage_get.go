package assessments

import (
	"context"
	"time"

	"bigroup-backend/internal/httpapi/dto"
)

// Get returns the full read-only definition (questions, options, effects) of a
// non-archived assessment; archived or unknown ids yield ErrNotFound.
func (r *ManageRepo) Get(ctx context.Context, id int64) (dto.ManageAssessment, error) {
	it, err := r.item(ctx, id)
	if err != nil {
		return dto.ManageAssessment{}, err
	}
	out := dto.ManageAssessment{
		ID: it.ID, Title: it.Title, Description: it.Description, Hidden: it.Hidden,
		ExpiresAt: it.ExpiresAt, Status: ManageStatus(it.Hidden, it.ExpiresAt, time.Now()),
		AttemptCount: it.AttemptCount, CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt,
		Questions: []dto.ManageQuestion{},
	}
	qrows, err := r.pool.Query(ctx, `SELECT id, position, text, type FROM questions
		WHERE assessment_id=$1 ORDER BY position, id`, id)
	if err != nil {
		return out, err
	}
	defer qrows.Close()
	qidx := map[int64]int{}
	for qrows.Next() {
		q := dto.ManageQuestion{Options: []dto.ManageOption{}}
		if err := qrows.Scan(&q.ID, &q.Position, &q.Text, &q.Type); err != nil {
			return out, err
		}
		qidx[q.ID] = len(out.Questions)
		out.Questions = append(out.Questions, q)
	}
	if err := qrows.Err(); err != nil {
		return out, err
	}
	qrows.Close()

	orows, err := r.pool.Query(ctx, `SELECT o.id, o.question_id, o.position, o.text
		FROM options o JOIN questions q ON q.id=o.question_id
		WHERE q.assessment_id=$1 ORDER BY o.position, o.id`, id)
	if err != nil {
		return out, err
	}
	defer orows.Close()
	type optRef struct{ q, o int }
	oidx := map[int64]optRef{}
	for orows.Next() {
		var qid int64
		o := dto.ManageOption{Effects: []dto.Effect{}}
		if err := orows.Scan(&o.ID, &qid, &o.Position, &o.Text); err != nil {
			return out, err
		}
		qi := qidx[qid]
		oidx[o.ID] = optRef{qi, len(out.Questions[qi].Options)}
		out.Questions[qi].Options = append(out.Questions[qi].Options, o)
	}
	if err := orows.Err(); err != nil {
		return out, err
	}
	orows.Close()

	erows, err := r.pool.Query(ctx, `SELECT e.option_id, e.characteristic_id, e.level, e.direction
		FROM option_effects e JOIN options o ON o.id=e.option_id
		JOIN questions q ON q.id=o.question_id
		WHERE q.assessment_id=$1 ORDER BY e.option_id, e.characteristic_id`, id)
	if err != nil {
		return out, err
	}
	defer erows.Close()
	for erows.Next() {
		var oid int64
		var e dto.Effect
		if err := erows.Scan(&oid, &e.CharacteristicID, &e.Level, &e.Direction); err != nil {
			return out, err
		}
		ref := oidx[oid]
		opt := &out.Questions[ref.q].Options[ref.o]
		opt.Effects = append(opt.Effects, e)
	}
	return out, erows.Err()
}
