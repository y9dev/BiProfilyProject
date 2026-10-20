package departments

import (
	"context"

	"bigroup-backend/internal/model"
)

// ListActive returns active departments ordered by name (register options).
func (r *repo) ListActive(ctx context.Context) ([]model.Department, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, active, created_at FROM departments WHERE active ORDER BY lower(name), id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Department{}
	for rows.Next() {
		var d model.Department
		if err := rows.Scan(&d.ID, &d.Name, &d.Active, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
