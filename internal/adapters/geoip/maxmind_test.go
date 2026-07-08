package geoip_test

import (
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/geoip"
)

func TestNewMaxMindResolver_ErrorsOnMissingFile(t *testing.T) {
	if _, err := geoip.NewMaxMindResolver("/does/not/exist.mmdb"); err == nil {
		t.Error("expected an error opening a nonexistent database file")
	}
}
