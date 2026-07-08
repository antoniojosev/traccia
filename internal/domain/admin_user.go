package domain

import "time"

// AdminUser is a human account for the /admin panel — a different trust
// model than ADMIN_TOKEN (which stays as the API's machine-to-machine
// credential for POST /api/v1/projects). The first account is created via
// a one-time setup flow; there's no open registration after that.
type AdminUser struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}
