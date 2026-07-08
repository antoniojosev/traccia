package usecase

import (
	"context"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

// GetMetadataBreakdown powers a plugin's registerPanel groupBy — the only
// thing that turns a panel's spec into actual computed data.
type GetMetadataBreakdown struct {
	Events ports.EventRepository
}

func NewGetMetadataBreakdown(events ports.EventRepository) *GetMetadataBreakdown {
	return &GetMetadataBreakdown{Events: events}
}

type GetMetadataBreakdownInput struct {
	ProjectID   string
	Type        domain.EventType
	EventName   string
	MetadataKey string
	Since       time.Time
	Until       time.Time
	Limit       int
}

func (uc *GetMetadataBreakdown) Execute(ctx context.Context, in GetMetadataBreakdownInput) ([]domain.NameCount, error) {
	if in.Until.IsZero() {
		in.Until = time.Now().UTC()
	}
	if in.Since.IsZero() {
		in.Since = in.Until.AddDate(0, 0, -7)
	}
	if in.Limit <= 0 || in.Limit > 50 {
		in.Limit = 10
	}
	return uc.Events.MetadataBreakdown(ctx, domain.StatsFilter{
		ProjectID: in.ProjectID,
		Since:     in.Since,
		Until:     in.Until,
	}, in.Type, in.EventName, in.MetadataKey, in.Limit)
}
