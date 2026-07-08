package domain

import "time"

// Visitor is the durable identity attached to a visitor_id via Identify.
// Unlike Event.Metadata (scoped to a single occurrence), these traits
// persist across every future event from the same visitor_id without being
// repeated on each track() call.
//
// Name is first-class (not buried in Properties) because it's the field
// used to tell known visitors apart from anonymous real traffic — e.g. an
// owner tagging their own browser as "Antonio (yo mismo)" so stats queries
// can exclude it.
type Visitor struct {
	ProjectID  string
	VisitorID  string
	Name       string
	Properties map[string]any
	UpdatedAt  time.Time
}
