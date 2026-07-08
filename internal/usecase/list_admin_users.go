package usecase

import (
	"context"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

type ListAdminUsers struct {
	Users ports.AdminUserRepository
}

func NewListAdminUsers(users ports.AdminUserRepository) *ListAdminUsers {
	return &ListAdminUsers{Users: users}
}

func (uc *ListAdminUsers) Execute(ctx context.Context) ([]domain.AdminUser, error) {
	return uc.Users.List(ctx)
}
