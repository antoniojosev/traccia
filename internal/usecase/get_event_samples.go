package usecase

import (
	"context"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

type GetEventSamples struct {
	Events ports.EventRepository
}

func NewGetEventSamples(events ports.EventRepository) *GetEventSamples {
	return &GetEventSamples{Events: events}
}

type GetEventSamplesInput struct {
	ProjectID string
	Type      domain.EventType
	Name      string
	Since     time.Time
	Until     time.Time
	Limit     int
}

func (uc *GetEventSamples) Execute(ctx context.Context, in GetEventSamplesInput) ([]domain.EventDetail, error) {
	if in.Until.IsZero() {
		in.Until = time.Now().UTC()
	}
	if in.Since.IsZero() {
		in.Since = in.Until.AddDate(0, 0, -7)
	}
	if in.Limit <= 0 || in.Limit > 200 {
		in.Limit = 50
	}
	return uc.Events.RecentByName(ctx, domain.StatsFilter{
		ProjectID: in.ProjectID,
		Since:     in.Since,
		Until:     in.Until,
	}, in.Type, in.Name, in.Limit)
}
