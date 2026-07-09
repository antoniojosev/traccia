package dashboard_test

import (
	"context"
	"errors"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
)

type fakeEventRepo struct {
	saved             []domain.Event
	samples           []domain.EventDetail
	metadataBreakdown []domain.NameCount
}

func (f *fakeEventRepo) Save(_ context.Context, event domain.Event) error {
	f.saved = append(f.saved, event)
	return nil
}

func (f *fakeEventRepo) Stats(_ context.Context, _ domain.StatsFilter) (domain.Stats, error) {
	return domain.Stats{
		TotalEvents:    int64(len(f.saved)),
		VisitsOverTime: []domain.TimeseriesPoint{{Bucket: time.Now(), Count: int64(len(f.saved))}},
	}, nil
}

func (f *fakeEventRepo) RecentByName(_ context.Context, _ domain.StatsFilter, _ domain.EventType, _ string, _ int) ([]domain.EventDetail, error) {
	return f.samples, nil
}

func (f *fakeEventRepo) MetadataBreakdown(_ context.Context, _ domain.StatsFilter, _ domain.EventType, _, _ string, _ int) ([]domain.NameCount, error) {
	return f.metadataBreakdown, nil
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
