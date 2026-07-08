package domain

import "time"

type StatsFilter struct {
	ProjectID string
	Since     time.Time
	Until     time.Time
	// ExcludeNamed drops events from visitors that have been Identify()'d
	// with a Name — e.g. filtering out the project owner's own traffic.
	ExcludeNamed bool
	// IncludeBots includes events whose UserAgentParser-detected device
	// type is "bot". Off by default: a crawler shouldn't inflate traffic
	// numbers unless you explicitly ask to see it.
	IncludeBots bool
}

type PathCount struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

type ReferrerCount struct {
	Referrer string `json:"referrer"`
	Count    int64  `json:"count"`
}

type TimeseriesPoint struct {
	Bucket time.Time `json:"bucket"`
	Count  int64     `json:"count"`
}

// NameCount is a generic (label, count) pair used for every breakdown that
// groups events by a single column: device type, browser, OS, or the name
// of a custom/error event.
type NameCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// EventDetail is a single event returned by a drill-down query — e.g.
// "show me the last 50 calculator_used events with their metadata".
type EventDetail struct {
	VisitorID string         `json:"visitor_id"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

// Stats is the aggregate returned to a project's dashboard/API consumer.
// JSON tags keep the wire format snake_case, consistent with the rest of
// the API (project_id, visitor_id, ...) instead of Go's default PascalCase.
type Stats struct {
	TotalEvents      int64             `json:"total_events"`
	UniqueVisitors   int64             `json:"unique_visitors"`
	TopPaths         []PathCount       `json:"top_paths"`
	TopReferrers     []ReferrerCount   `json:"top_referrers"`
	VisitsOverTime   []TimeseriesPoint `json:"visits_over_time"`
	DeviceTypes      []NameCount       `json:"device_types"`
	Browsers         []NameCount       `json:"browsers"`
	OperatingSystems []NameCount       `json:"operating_systems"`
	CustomEventNames []NameCount       `json:"custom_event_names"`
	ErrorEventNames  []NameCount       `json:"error_event_names"`
}
