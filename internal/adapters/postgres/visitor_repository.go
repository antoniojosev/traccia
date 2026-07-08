package postgres

import (
	"context"
	"encoding/json"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VisitorRepository struct {
	pool *pgxpool.Pool
}

func NewVisitorRepository(pool *pgxpool.Pool) *VisitorRepository {
	return &VisitorRepository{pool: pool}
}

// Upsert keeps any previously set Name if this call doesn't provide a new
// one, and merges (rather than replaces) Properties — so calling Identify
// again with just one new trait doesn't wipe out the others.
func (r *VisitorRepository) Upsert(ctx context.Context, visitor domain.Visitor) error {
	properties := visitor.Properties
	if properties == nil {
		properties = map[string]any{}
	}
	propertiesJSON, err := json.Marshal(properties)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO visitors (project_id, visitor_id, name, properties, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (project_id, visitor_id) DO UPDATE SET
			name       = CASE WHEN EXCLUDED.name = '' THEN visitors.name ELSE EXCLUDED.name END,
			properties = visitors.properties || EXCLUDED.properties,
			updated_at = EXCLUDED.updated_at
	`, visitor.ProjectID, visitor.VisitorID, visitor.Name, propertiesJSON, visitor.UpdatedAt)
	return err
}
