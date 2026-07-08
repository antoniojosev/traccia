package geoip

import (
	"net"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/oschwald/geoip2-golang"
)

// MaxMindResolver reads a local GeoLite2/GeoIP2 City .mmdb file — no
// network calls, no external service, consistent with "self-hosted, no
// external dependencies." It's not bundled (the database itself requires
// a free MaxMind account to download and its license doesn't permit
// redistribution): point GEOIP_DB_PATH at your own copy to enable it.
type MaxMindResolver struct {
	reader *geoip2.Reader
}

func NewMaxMindResolver(dbPath string) (*MaxMindResolver, error) {
	reader, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &MaxMindResolver{reader: reader}, nil
}

func (r *MaxMindResolver) Close() error {
	return r.reader.Close()
}

// Resolve never errors outward — a bad/unparseable IP or a lookup miss
// (private ranges, reserved addresses, database gaps) just yields an
// empty GeoInfo, the same as the no-op resolver.
func (r *MaxMindResolver) Resolve(ip string) domain.GeoInfo {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return domain.GeoInfo{}
	}

	record, err := r.reader.City(parsed)
	if err != nil {
		return domain.GeoInfo{}
	}

	return domain.GeoInfo{
		Country: record.Country.Names["en"],
		City:    record.City.Names["en"],
	}
}
