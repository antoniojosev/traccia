package domain

import "time"

type EventType string

const (
	EventTypePageview EventType = "pageview"
	EventTypeCustom   EventType = "custom"
	EventTypeError    EventType = "error"
)

// DeviceInfo is produced by a ports.UserAgentParser implementation.
type DeviceInfo struct {
	DeviceType string // desktop | mobile | tablet | bot | unknown
	Browser    string
	OS         string
}

// GeoInfo is produced by a ports.GeoResolver implementation.
type GeoInfo struct {
	Country string
	City    string
}

// Event is the single fact table Traccia records. A pageview, a custom
// product event and an error report are all Events distinguished by Type.
type Event struct {
	ID           int64
	ProjectID    string
	VisitorID    string // stable per-browser UUID, set via cookie by the SDK
	Type         EventType
	Name         string // required for custom/error, empty for pageview
	Path         string
	Referrer     string
	IPAnonymized string // last octet/segment zeroed before it ever reaches storage
	Device       DeviceInfo
	Geo          GeoInfo
	Metadata     map[string]any
	CreatedAt    time.Time
}
