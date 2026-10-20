package assessments

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/httpapi/dto"
)

// Creator inserts new assessment definitions (structure is immutable afterwards).
type Creator struct{ pool *pgxpool.Pool }

func NewCreator(pool *pgxpool.Pool) *Creator { return &Creator{pool: pool} }

// KnownCharacteristics returns the set of existing characteristic ids.
func (c *Creator) KnownCharacteristics(ctx context.Context) (map[int64]bool, error) {
	rows, err := c.pool.Query(ctx, `SELECT id FROM characteristics`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// Create inserts the assessment, questions, options and effects in one transaction
// and returns the new assessment id. The request must already be validated.
func (c *Creator) Create(ctx context.Context, createdBy int64, req dto.CreateAssessmentRequest) (int64, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO assessments (title, description, hidden, expires_at, created_by)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		req.Title, req.Description, req.Hidden, req.ExpiresAt, createdBy).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert assessment: %w", err)
	}
	for qi, q := range req.Questions {
		var qid int64
		if err := tx.QueryRow(ctx, `INSERT INTO questions (assessment_id, position, text, type)
			VALUES ($1,$2,$3,$4) RETURNING id`, id, qi, q.Text, q.Type).Scan(&qid); err != nil {
			return 0, fmt.Errorf("insert question: %w", err)
		}
		for oi, o := range q.Options {
			var oid int64
			if err := tx.QueryRow(ctx, `INSERT INTO options (question_id, position, text)
				VALUES ($1,$2,$3) RETURNING id`, qid, oi, o.Text).Scan(&oid); err != nil {
				return 0, fmt.Errorf("insert option: %w", err)
			}
			if len(o.Effects) == 0 {
				continue
			}
			batch := &pgx.Batch{}
			for _, e := range o.Effects {
				batch.Queue(`INSERT INTO option_effects (option_id, characteristic_id, level, direction)
					VALUES ($1,$2,$3,$4)`, oid, e.CharacteristicID, e.Level, e.Direction)
			}
			br := tx.SendBatch(ctx, batch)
			for range o.Effects {
				if _, err := br.Exec(); err != nil {
					_ = br.Close()
					return 0, fmt.Errorf("insert effect: %w", err)
				}
			}
			if err := br.Close(); err != nil {
				return 0, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}
