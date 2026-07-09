package usecase

import (
	"context"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

// GetAdminUser looks an admin account up by its own ID — used to render a
// delete-confirmation page and to re-verify a session's subject still
// exists in the database.
type GetAdminUser struct {
	Users ports.AdminUserRepository
}

func NewGetAdminUser(users ports.AdminUserRepository) *GetAdminUser {
	return &GetAdminUser{Users: users}
}

func (uc *GetAdminUser) Execute(ctx context.Context, id string) (domain.AdminUser, error) {
	return uc.Users.FindByID(ctx, id)
}
