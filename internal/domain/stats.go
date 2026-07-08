package domain

import "time"

type StatsFilter struct {
	ProjectID string
	Since     time.Time
	Until     time.Time
	// ExcludeNamed drops events from visitors that have been Identify()'d
	// with a Name — e.g. filtering out the project owner's own traffic.
	ExcludeNamed bool
}

type PathCount struct {
	Path  string
	Count int64
}

type ReferrerCount struct {
	Referrer string
	Count    int64
}

type TimeseriesPoint struct {
	Bucket time.Time
	Count  int64
}

// Stats is the aggregate returned to a project's dashboard/API consumer.
type Stats struct {
	TotalEvents    int64
	UniqueVisitors int64
	TopPaths       []PathCount
	TopReferrers   []ReferrerCount
	VisitsOverTime []TimeseriesPoint
}
