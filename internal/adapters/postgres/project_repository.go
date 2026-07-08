package postgres

import (
	"context"
	"errors"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrProjectNotFound = errors.New("postgres: project not found")

type ProjectRepository struct {
	pool *pgxpool.Pool
}

func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

func (r *ProjectRepository) Create(ctx context.Context, project domain.Project) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO projects (id, name, domain, api_key_hash, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, project.ID, project.Name, project.Domain, project.APIKeyHash, project.CreatedAt)
	return err
}

func (r *ProjectRepository) FindByID(ctx context.Context, id string) (domain.Project, error) {
	return r.scanOne(ctx, `SELECT id, name, domain, api_key_hash, created_at FROM projects WHERE id = $1`, id)
}

func (r *ProjectRepository) FindByAPIKeyHash(ctx context.Context, apiKeyHash string) (domain.Project, error) {
	return r.scanOne(ctx, `SELECT id, name, domain, api_key_hash, created_at FROM projects WHERE api_key_hash = $1`, apiKeyHash)
}

func (r *ProjectRepository) scanOne(ctx context.Context, query, arg string) (domain.Project, error) {
	var p domain.Project
	err := r.pool.QueryRow(ctx, query, arg).Scan(&p.ID, &p.Name, &p.Domain, &p.APIKeyHash, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, ErrProjectNotFound
	}
	return p, err
}

// Delete cascades to that project's events and visitors via the foreign
// keys' ON DELETE CASCADE — deleting a project deletes everything it ever
// recorded, irreversibly. (plugin_kv is scoped by plugin name, not
// project, so it's untouched.)
func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

func (r *ProjectRepository) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, domain, api_key_hash, created_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Domain, &p.APIKeyHash, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
