package usecase

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

var ErrInvalidEvent = errors.New("traccia: invalid event")

type TrackEventInput struct {
	ProjectID string
	VisitorID string
	Type      domain.EventType
	Name      string
	Path      string
	Referrer  string
	IP        string
	UserAgent string
	Metadata  map[string]any
}

type TrackEvent struct {
	Events ports.EventRepository
	UA     ports.UserAgentParser
	Geo    ports.GeoResolver
}

func NewTrackEvent(events ports.EventRepository, ua ports.UserAgentParser, geo ports.GeoResolver) *TrackEvent {
	return &TrackEvent{Events: events, UA: ua, Geo: geo}
}

func (uc *TrackEvent) Execute(ctx context.Context, in TrackEventInput) error {
	if in.ProjectID == "" || in.VisitorID == "" {
		return ErrInvalidEvent
	}
	if in.Type == "" {
		in.Type = domain.EventTypePageview
	}
	if (in.Type == domain.EventTypeCustom || in.Type == domain.EventTypeError) && in.Name == "" {
		return ErrInvalidEvent
	}

	geo := uc.Geo.Resolve(in.IP)
	device := uc.UA.Parse(in.UserAgent)

	event := domain.Event{
		ProjectID:    in.ProjectID,
		VisitorID:    in.VisitorID,
		Type:         in.Type,
		Name:         in.Name,
		Path:         in.Path,
		Referrer:     in.Referrer,
		IPAnonymized: anonymizeIP(in.IP),
		Device:       device,
		Geo:          geo,
		Metadata:     in.Metadata,
		CreatedAt:    time.Now().UTC(),
	}

	return uc.Events.Save(ctx, event)
}

// anonymizeIP zeroes the last octet (IPv4) or last 80 bits (IPv6) before the
// address ever reaches storage, similar to Plausible/Matomo. Geo resolution
// must happen on the original IP, before this is called.
func anonymizeIP(raw string) string {
	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		v4[3] = 0
		return v4.String()
	}
	v6 := ip.To16()
	if v6 == nil {
		return ""
	}
	for i := 6; i < len(v6); i++ {
		v6[i] = 0
	}
	return v6.String()
}
