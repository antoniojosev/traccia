package usecase

import (
	"context"
	"errors"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

var ErrAdminInvalidCredentials = errors.New("usecase: invalid admin username or password")

type AuthenticateAdminUser struct {
	Users  ports.AdminUserRepository
	Hasher ports.PasswordHasher
}

func NewAuthenticateAdminUser(users ports.AdminUserRepository, hasher ports.PasswordHasher) *AuthenticateAdminUser {
	return &AuthenticateAdminUser{Users: users, Hasher: hasher}
}

func (uc *AuthenticateAdminUser) Execute(ctx context.Context, username, password string) (domain.AdminUser, error) {
	user, err := uc.Users.FindByUsername(ctx, username)
	if err != nil {
		return domain.AdminUser{}, ErrAdminInvalidCredentials
	}
	if !uc.Hasher.Verify(user.PasswordHash, password) {
		return domain.AdminUser{}, ErrAdminInvalidCredentials
	}
	return user, nil
}
