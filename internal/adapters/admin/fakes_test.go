package admin_test

import (
	"context"
	"errors"

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

type fakeKeyHasher struct{}

func (fakeKeyHasher) Generate() (plainKey string, hash string, err error) {
	return "plain-key", "hash-of-plain-key", nil
}

func (fakeKeyHasher) Hash(plainKey string) string {
	return "hash-of-" + plainKey
}
