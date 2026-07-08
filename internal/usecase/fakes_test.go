package usecase_test

import (
	"context"
	"errors"

	"github.com/antoniojosev/traccia/internal/domain"
)

type fakeEventRepo struct {
	saved []domain.Event
}

func (f *fakeEventRepo) Save(_ context.Context, event domain.Event) error {
	f.saved = append(f.saved, event)
	return nil
}

func (f *fakeEventRepo) Stats(_ context.Context, _ domain.StatsFilter) (domain.Stats, error) {
	return domain.Stats{}, nil
}

type fakeUAParser struct{}

func (fakeUAParser) Parse(_ string) domain.DeviceInfo {
	return domain.DeviceInfo{DeviceType: "desktop", Browser: "chrome", OS: "linux"}
}

type fakeGeoResolver struct{}

func (fakeGeoResolver) Resolve(_ string) domain.GeoInfo {
	return domain.GeoInfo{Country: "VE", City: "Caracas"}
}

type fakeProjectRepo struct {
	byID   map[string]domain.Project
	byHash map[string]domain.Project
}

func newFakeProjectRepo() *fakeProjectRepo {
	return &fakeProjectRepo{byID: map[string]domain.Project{}, byHash: map[string]domain.Project{}}
}

func (f *fakeProjectRepo) Create(_ context.Context, project domain.Project) error {
	f.byID[project.ID] = project
	f.byHash[project.APIKeyHash] = project
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

type fakeKeyHasher struct{}

func (fakeKeyHasher) Generate() (plainKey string, hash string, err error) {
	return "plain-key", "hash-of-plain-key", nil
}

func (fakeKeyHasher) Hash(plainKey string) string {
	return "hash-of-" + plainKey
}

type fakeVisitorRepo struct {
	upserted []domain.Visitor
}

func (f *fakeVisitorRepo) Upsert(_ context.Context, visitor domain.Visitor) error {
	f.upserted = append(f.upserted, visitor)
	return nil
}
