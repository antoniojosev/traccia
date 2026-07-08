package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	AdminToken         string
	RateLimitPerMinute int
	// LoginRateLimitPerMinute guards /dashboard/login, /admin/login and
	// /admin/setup — deliberately much stricter than RateLimitPerMinute,
	// since a login attempt should never be as frequent as a pageview.
	LoginRateLimitPerMinute int
	// SessionSecret signs dashboard login cookies. Empty means main.go
	// derives one from AdminToken — fine to get started, but set your own
	// for production so dashboard sessions don't depend on that secret too.
	SessionSecret string
	// PluginsDir is scanned for *.js plugins at boot. A missing directory
	// is not an error — plugins are optional.
	PluginsDir string
	// GeoIPDatabasePath points at a local MaxMind GeoLite2/GeoIP2 City
	// .mmdb file. Empty (the default) means country/city stay unresolved
	// — the database itself isn't bundled (needs a free MaxMind account
	// and its license doesn't permit redistribution).
	GeoIPDatabasePath string
}

func Load() Config {
	return Config{
		Port:                    getEnv("PORT", "8080"),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		AdminToken:              getEnv("ADMIN_TOKEN", ""),
		RateLimitPerMinute:      getEnvInt("RATE_LIMIT_PER_MINUTE", 120),
		LoginRateLimitPerMinute: getEnvInt("LOGIN_RATE_LIMIT_PER_MINUTE", 10),
		SessionSecret:           getEnv("SESSION_SECRET", ""),
		PluginsDir:              getEnv("PLUGINS_DIR", "./plugins"),
		GeoIPDatabasePath:       getEnv("GEOIP_DB_PATH", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
