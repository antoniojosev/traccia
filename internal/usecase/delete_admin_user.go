package usecase

import (
	"context"
	"errors"

	"github.com/antoniojosev/traccia/internal/ports"
)

// ErrCannotDeleteLastAdmin guards against locking everyone out of the
// panel — there must always be at least one admin account able to log in.
var ErrCannotDeleteLastAdmin = errors.New("usecase: cannot delete the last remaining admin account")

type DeleteAdminUser struct {
	Users ports.AdminUserRepository
}

func NewDeleteAdminUser(users ports.AdminUserRepository) *DeleteAdminUser {
	return &DeleteAdminUser{Users: users}
}

func (uc *DeleteAdminUser) Execute(ctx context.Context, id string) error {
	count, err := uc.Users.Count(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrCannotDeleteLastAdmin
	}
	return uc.Users.Delete(ctx, id)
}
