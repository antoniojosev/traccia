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

// Stats is the aggregate returned to a project's dashboard/API consumer.
// JSON tags keep the wire format snake_case, consistent with the rest of
// the API (project_id, visitor_id, ...) instead of Go's default PascalCase.
type Stats struct {
	TotalEvents    int64             `json:"total_events"`
	UniqueVisitors int64             `json:"unique_visitors"`
	TopPaths       []PathCount       `json:"top_paths"`
	TopReferrers   []ReferrerCount   `json:"top_referrers"`
	VisitsOverTime []TimeseriesPoint `json:"visits_over_time"`
}
