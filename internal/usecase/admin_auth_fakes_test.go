package usecase_test

import (
	"context"
	"errors"

	"github.com/antoniojosev/traccia/internal/domain"
)

type fakeAdminUserRepo struct {
	byUsername map[string]domain.AdminUser
}

func newFakeAdminUserRepo() *fakeAdminUserRepo {
	return &fakeAdminUserRepo{byUsername: map[string]domain.AdminUser{}}
}

func (f *fakeAdminUserRepo) Create(_ context.Context, user domain.AdminUser) error {
	f.byUsername[user.Username] = user
	return nil
}

func (f *fakeAdminUserRepo) FindByUsername(_ context.Context, username string) (domain.AdminUser, error) {
	u, ok := f.byUsername[username]
	if !ok {
		return domain.AdminUser{}, errors.New("not found")
	}
	return u, nil
}

func (f *fakeAdminUserRepo) Count(_ context.Context) (int, error) {
	return len(f.byUsername), nil
}

func (f *fakeAdminUserRepo) List(_ context.Context) ([]domain.AdminUser, error) {
	out := make([]domain.AdminUser, 0, len(f.byUsername))
	for _, u := range f.byUsername {
		out = append(out, u)
	}
	return out, nil
}

type fakePasswordHasher struct{}

func (fakePasswordHasher) Hash(plain string) (string, error) {
	return "hashed:" + plain, nil
}

func (fakePasswordHasher) Verify(hash, plain string) bool {
	return hash == "hashed:"+plain
}
