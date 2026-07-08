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
	// SessionSecret signs dashboard login cookies. Empty means main.go
	// derives one from AdminToken — fine to get started, but set your own
	// for production so dashboard sessions don't depend on that secret too.
	SessionSecret string
	// PluginsDir is scanned for *.js plugins at boot. A missing directory
	// is not an error — plugins are optional.
	PluginsDir string
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		AdminToken:         getEnv("ADMIN_TOKEN", ""),
		RateLimitPerMinute: getEnvInt("RATE_LIMIT_PER_MINUTE", 120),
		SessionSecret:      getEnv("SESSION_SECRET", ""),
		PluginsDir:         getEnv("PLUGINS_DIR", "./plugins"),
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
