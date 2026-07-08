// Package geoip provides Traccia's default GeoResolver: a no-op. Plug in a
// MaxMind/IP2Location-backed adapter implementing ports.GeoResolver to
// enable country/city resolution without touching any usecase.
package geoip

import "github.com/antoniojosev/traccia/internal/domain"

type NoopResolver struct{}

func NewNoopResolver() *NoopResolver {
	return &NoopResolver{}
}

func (r *NoopResolver) Resolve(ip string) domain.GeoInfo {
	return domain.GeoInfo{}
}
