package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
	"github.com/google/uuid"
)

var ErrAdminUsernameTaken = errors.New("usecase: that admin username is already taken")

// AddAdminUser creates an additional admin account for an already
// authenticated session — unlike RegisterAdminUser, it never checks
// whether the panel has zero accounts, since it's only reachable by
// someone who already has one.
type AddAdminUser struct {
	Users  ports.AdminUserRepository
	Hasher ports.PasswordHasher
}

func NewAddAdminUser(users ports.AdminUserRepository, hasher ports.PasswordHasher) *AddAdminUser {
	return &AddAdminUser{Users: users, Hasher: hasher}
}

func (uc *AddAdminUser) Execute(ctx context.Context, username, password string) (domain.AdminUser, error) {
	if len(username) < 3 || len(password) < minAdminPasswordChars {
		return domain.AdminUser{}, ErrAdminInvalidInput
	}

	if _, err := uc.Users.FindByUsername(ctx, username); err == nil {
		return domain.AdminUser{}, ErrAdminUsernameTaken
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
