package usecase

import (
	"context"

	"github.com/antoniojosev/traccia/internal/ports"
)

// NeedsAdminSetup reports whether the one-time admin account setup has
// not happened yet — the admin panel routes to /admin/setup instead of
// /admin/login until this is false.
type NeedsAdminSetup struct {
	Users ports.AdminUserRepository
}

func NewNeedsAdminSetup(users ports.AdminUserRepository) *NeedsAdminSetup {
	return &NeedsAdminSetup{Users: users}
}

func (uc *NeedsAdminSetup) Execute(ctx context.Context) (bool, error) {
	count, err := uc.Users.Count(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}
