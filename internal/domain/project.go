package domain

import "time"

// Project is a tenant: one entry per app/site that sends events to Traccia.
type Project struct {
	ID         string
	Name       string
	Domain     string
	APIKeyHash string
	CreatedAt  time.Time
}
