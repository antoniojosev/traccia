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

func (f *fakeEventRepo) RecentByName(_ context.Context, _ domain.StatsFilter, _ domain.EventType, _ string, _ int) ([]domain.EventDetail, error) {
	return nil, nil
}

func (f *fakeEventRepo) MetadataBreakdown(_ context.Context, _ domain.StatsFilter, _ domain.EventType, _, _ string, _ int) ([]domain.NameCount, error) {
	return nil, nil
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
