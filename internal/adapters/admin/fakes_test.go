package admin_test

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"

	"github.com/antoniojosev/traccia/internal/domain"
)

type fakeProjectRepo struct {
	byID   map[string]domain.Project
	byHash map[string]domain.Project
	order  []string
}

func newFakeProjectRepo() *fakeProjectRepo {
	return &fakeProjectRepo{byID: map[string]domain.Project{}, byHash: map[string]domain.Project{}}
}

func (f *fakeProjectRepo) Create(_ context.Context, project domain.Project) error {
	f.byID[project.ID] = project
	f.byHash[project.APIKeyHash] = project
	f.order = append(f.order, project.ID)
	return nil
}

func (f *fakeProjectRepo) FindByID(_ context.Context, id string) (domain.Project, error) {
	p, ok := f.byID[id]
	if !ok {
		return domain.Project{}, errors.New("not found")
	}
	return p, nil
}

func (f *fakeProjectRepo) FindByAPIKeyHash(_ context.Context, hash string) (domain.Project, error) {
	p, ok := f.byHash[hash]
	if !ok {
		return domain.Project{}, errors.New("not found")
	}
	return p, nil
}

func (f *fakeProjectRepo) List(_ context.Context) ([]domain.Project, error) {
	out := make([]domain.Project, 0, len(f.order))
	for _, id := range f.order {
		out = append(out, f.byID[id])
	}
	return out, nil
}

func (f *fakeProjectRepo) Delete(_ context.Context, id string) error {
	if p, ok := f.byID[id]; ok {
		delete(f.byHash, p.APIKeyHash)
	}
	delete(f.byID, id)
	for i, oid := range f.order {
		if oid == id {
			f.order = append(f.order[:i], f.order[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeProjectRepo) Update(_ context.Context, project domain.Project) error {
	existing, ok := f.byID[project.ID]
	if !ok {
		return errors.New("not found")
	}
	delete(f.byHash, existing.APIKeyHash)
	f.byID[project.ID] = project
	f.byHash[project.APIKeyHash] = project
	return nil
}

// fakeKeyCounter backs fakeKeyHasher.Generate with a package-level counter
// (rather than per-instance) so a distinct key comes back every call even
// when tests construct separate fakeKeyHasher values for seeding vs. for
// the handler under test — a fixed return value would make "the old key no
// longer works" assertions vacuously true.
var fakeKeyCounter int32

type fakeKeyHasher struct{}

func (fakeKeyHasher) Generate() (plainKey string, hash string, err error) {
	n := atomic.AddInt32(&fakeKeyCounter, 1)
	plainKey = "plain-key-" + strconv.Itoa(int(n))
	return plainKey, "hash-of-" + plainKey, nil
}

func (fakeKeyHasher) Hash(plainKey string) string {
	return "hash-of-" + plainKey
}

type fakeAdminUserRepo struct {
	byUsername map[string]domain.AdminUser
	order      []string
}

func newFakeAdminUserRepo() *fakeAdminUserRepo {
	return &fakeAdminUserRepo{byUsername: map[string]domain.AdminUser{}}
}

func (f *fakeAdminUserRepo) Create(_ context.Context, user domain.AdminUser) error {
	f.byUsername[user.Username] = user
	f.order = append(f.order, user.Username)
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
	out := make([]domain.AdminUser, 0, len(f.order))
	for _, username := range f.order {
		out = append(out, f.byUsername[username])
	}
	return out, nil
}

func (f *fakeAdminUserRepo) FindByID(_ context.Context, id string) (domain.AdminUser, error) {
	for _, u := range f.byUsername {
		if u.ID == id {
			return u, nil
		}
	}
	return domain.AdminUser{}, errors.New("not found")
}

func (f *fakeAdminUserRepo) Delete(_ context.Context, id string) error {
	for username, u := range f.byUsername {
		if u.ID == id {
			delete(f.byUsername, username)
			for i, oid := range f.order {
				if oid == username {
					f.order = append(f.order[:i], f.order[i+1:]...)
					break
				}
			}
			return nil
		}
	}
	return errors.New("not found")
}

type fakePasswordHasher struct{}

func (fakePasswordHasher) Hash(plain string) (string, error) {
	return "hashed:" + plain, nil
}

func (fakePasswordHasher) Verify(hash, plain string) bool {
	return hash == "hashed:"+plain
}
