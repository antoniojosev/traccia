package usecase

import (
	"context"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

type GetStats struct {
	Events ports.EventRepository
}

func NewGetStats(events ports.EventRepository) *GetStats {
	return &GetStats{Events: events}
}

type GetStatsInput struct {
	ProjectID    string
	Since        time.Time
	Until        time.Time
	ExcludeNamed bool
}

func (uc *GetStats) Execute(ctx context.Context, in GetStatsInput) (domain.Stats, error) {
	if in.Until.IsZero() {
		in.Until = time.Now().UTC()
	}
	if in.Since.IsZero() {
		in.Since = in.Until.AddDate(0, 0, -7)
	}
	return uc.Events.Stats(ctx, domain.StatsFilter{
		ProjectID:    in.ProjectID,
		Since:        in.Since,
		Until:        in.Until,
		ExcludeNamed: in.ExcludeNamed,
	})
}
