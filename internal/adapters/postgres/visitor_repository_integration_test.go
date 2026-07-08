//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/antoniojosev/traccia/internal/adapters/postgres"
	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/google/uuid"
)

func TestVisitorRepository_UpsertMergesPropertiesAndPreservesName(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)
	visitors := postgres.NewVisitorRepository(pool)

	project := createTestProject(t, ctx, projects)
	visitorID := uuid.NewString()
	now := time.Now().UTC()

	if err := visitors.Upsert(ctx, domain.Visitor{
		ProjectID: project.ID, VisitorID: visitorID, Name: "Antonio (yo mismo)",
		Properties: map[string]any{"plan": "pro"}, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Second call carries no name and a different property — it should
	// merge properties (not replace them) and keep the name already set.
	if err := visitors.Upsert(ctx, domain.Visitor{
		ProjectID: project.ID, VisitorID: visitorID,
		Properties: map[string]any{"referral_source": "twitter"}, UpdatedAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var name string
	var propertiesJSON []byte
	err := pool.QueryRow(ctx,
		`SELECT name, properties FROM visitors WHERE project_id = $1 AND visitor_id = $2`,
		project.ID, visitorID,
	).Scan(&name, &propertiesJSON)
	if err != nil {
		t.Fatalf("reading back visitor: %v", err)
	}

	if name != "Antonio (yo mismo)" {
		t.Errorf("expected name to be preserved across upserts, got %q", name)
	}

	var properties map[string]any
	if err := json.Unmarshal(propertiesJSON, &properties); err != nil {
		t.Fatalf("unmarshaling properties: %v", err)
	}
	if properties["plan"] != "pro" || properties["referral_source"] != "twitter" {
		t.Errorf("expected merged properties from both upserts, got %+v", properties)
	}
}
