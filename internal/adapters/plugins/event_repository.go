package plugins

import (
	"context"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

// EventRepositoryDecorator wraps a real ports.EventRepository and runs
// every plugin's onEvent hook before Save. This is the entire integration
// point between the plugin system and the rest of Traccia — usecases keep
// depending on ports.EventRepository and have no idea plugins exist.
type EventRepositoryDecorator struct {
	inner   ports.EventRepository
	manager *Manager
}

func NewEventRepositoryDecorator(inner ports.EventRepository, manager *Manager) *EventRepositoryDecorator {
	return &EventRepositoryDecorator{inner: inner, manager: manager}
}

func (d *EventRepositoryDecorator) Save(ctx context.Context, event domain.Event) error {
	event, keep := d.manager.RunOnEvent(event)
	if !keep {
		return nil
	}
	return d.inner.Save(ctx, event)
}

func (d *EventRepositoryDecorator) Stats(ctx context.Context, filter domain.StatsFilter) (domain.Stats, error) {
	return d.inner.Stats(ctx, filter)
}

func (d *EventRepositoryDecorator) RecentByName(ctx context.Context, filter domain.StatsFilter, eventType domain.EventType, name string, limit int) ([]domain.EventDetail, error) {
	return d.inner.RecentByName(ctx, filter, eventType, name, limit)
}
