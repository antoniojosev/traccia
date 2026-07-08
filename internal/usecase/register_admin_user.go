package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
	"github.com/google/uuid"
)

const minAdminPasswordChars = 8

var (
	ErrAdminSetupClosed  = errors.New("usecase: admin setup already completed")
	ErrAdminInvalidInput = errors.New("usecase: invalid username or password")
)

type RegisterAdminUser struct {
	Users  ports.AdminUserRepository
	Hasher ports.PasswordHasher
}

func NewRegisterAdminUser(users ports.AdminUserRepository, hasher ports.PasswordHasher) *RegisterAdminUser {
	return &RegisterAdminUser{Users: users, Hasher: hasher}
}

// Execute creates the first admin account. It refuses once any account
// already exists — there's no open registration, only a one-time setup,
// so a stray request to this endpoint can't add an attacker's own account
// once a real one is in place.
func (uc *RegisterAdminUser) Execute(ctx context.Context, username, password string) (domain.AdminUser, error) {
	if len(username) < 3 || len(password) < minAdminPasswordChars {
		return domain.AdminUser{}, ErrAdminInvalidInput
	}

	count, err := uc.Users.Count(ctx)
	if err != nil {
		return domain.AdminUser{}, err
	}
	if count > 0 {
		return domain.AdminUser{}, ErrAdminSetupClosed
	}

	hash, err := uc.Hasher.Hash(password)
	if err != nil {
		return domain.AdminUser{}, err
	}

	user := domain.AdminUser{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := uc.Users.Create(ctx, user); err != nil {
		return domain.AdminUser{}, err
	}
	return user, nil
}
