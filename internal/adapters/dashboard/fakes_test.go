package dashboard_test

import (
	"context"
	"errors"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
)

type fakeEventRepo struct {
	saved   []domain.Event
	samples []domain.EventDetail
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
