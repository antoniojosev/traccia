package postgres

import (
	"context"
	"errors"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAdminUserNotFound = errors.New("postgres: admin user not found")

type AdminUserRepository struct {
	pool *pgxpool.Pool
}

func NewAdminUserRepository(pool *pgxpool.Pool) *AdminUserRepository {
	return &AdminUserRepository{pool: pool}
}

func (r *AdminUserRepository) Create(ctx context.Context, user domain.AdminUser) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO admin_users (id, username, password_hash, created_at)
		VALUES ($1, $2, $3, $4)
	`, user.ID, user.Username, user.PasswordHash, user.CreatedAt)
	return err
}

func (r *AdminUserRepository) FindByUsername(ctx context.Context, username string) (domain.AdminUser, error) {
	var u domain.AdminUser
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at FROM admin_users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminUser{}, ErrAdminUserNotFound
	}
	return u, err
}

func (r *AdminUserRepository) FindByID(ctx context.Context, id string) (domain.AdminUser, error) {
	var u domain.AdminUser
	err := r.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at FROM admin_users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminUser{}, ErrAdminUserNotFound
	}
	return u, err
}

func (r *AdminUserRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM admin_users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAdminUserNotFound
	}
	return nil
}

func (r *AdminUserRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count)
	return count, err
}

func (r *AdminUserRepository) List(ctx context.Context) ([]domain.AdminUser, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, username, password_hash, created_at FROM admin_users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AdminUser
	for rows.Next() {
		var u domain.AdminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
