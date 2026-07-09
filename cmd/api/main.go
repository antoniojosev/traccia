package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/antoniojosev/traccia/internal/adapters/admin"
	"github.com/antoniojosev/traccia/internal/adapters/apikey"
	"github.com/antoniojosev/traccia/internal/adapters/dashboard"
	"github.com/antoniojosev/traccia/internal/adapters/geoip"
	"github.com/antoniojosev/traccia/internal/adapters/httpapi"
	"github.com/antoniojosev/traccia/internal/adapters/password"
	"github.com/antoniojosev/traccia/internal/adapters/plugins"
	"github.com/antoniojosev/traccia/internal/adapters/postgres"
	"github.com/antoniojosev/traccia/internal/adapters/ratelimit"
	"github.com/antoniojosev/traccia/internal/adapters/session"
	"github.com/antoniojosev/traccia/internal/adapters/useragent"
	"github.com/antoniojosev/traccia/internal/adapters/webui"
	"github.com/antoniojosev/traccia/internal/config"
	"github.com/antoniojosev/traccia/internal/ports"
	"github.com/antoniojosev/traccia/internal/usecase"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	if cfg.AdminToken == "" {
		slog.Warn("ADMIN_TOKEN is not set, POST /api/v1/projects is unreachable")
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = deriveSessionSecret(cfg.AdminToken)
		slog.Warn("SESSION_SECRET is not set, deriving one from ADMIN_TOKEN — set your own for production")
	}

	ctx := context.Background()
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connecting to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Default adapters — swap any of these for your own implementation of
	// the matching port without touching a usecase.
	rawEvents := postgres.NewEventRepository(pool)
	projects := postgres.NewProjectRepository(pool)
	visitors := postgres.NewVisitorRepository(pool)
	uaParser := useragent.NewHeuristicParser()
	geoResolver := loadGeoResolver(cfg.GeoIPDatabasePath)
	keyHasher := apikey.NewSHA256Hasher()
	pluginKV := postgres.NewPluginKVRepository(pool)
	adminUsers := postgres.NewAdminUserRepository(pool)
	passwordHasher := password.NewBcryptHasher()

	pluginManager, err := plugins.Load(cfg.PluginsDir, pluginKV)
	if err != nil {
		slog.Error("loading plugins", "dir", cfg.PluginsDir, "error", err)
		os.Exit(1)
	}

	// Every usecase depends on ports.EventRepository, not Postgres directly
	// — this decorator is the entire integration point for plugins' onEvent
	// hook, and no usecase needs to know it exists.
	events := plugins.NewEventRepositoryDecorator(rawEvents, pluginManager)

	auth := usecase.NewAuthenticateProject(projects, keyHasher)
	getStats := usecase.NewGetStats(events)

	apiRouter := httpapi.NewRouter(httpapi.Deps{
		AdminToken:      cfg.AdminToken,
		Auth:            auth,
		CreateProject:   usecase.NewCreateProject(projects, keyHasher),
		TrackEvent:      usecase.NewTrackEvent(events, uaParser, geoResolver),
		IdentifyVisitor: usecase.NewIdentifyVisitor(visitors),
		GetStats:        getStats,
		RateLimiter:     ratelimit.New(cfg.RateLimitPerMinute),
		Ping:            pool.Ping,
	})

	dashboardSessions := session.New(cfg.SessionSecret, "traccia_session", "/dashboard")
	adminSessions := session.New(cfg.SessionSecret, "traccia_admin_session", "/admin")
	dashboardHandler := dashboard.NewHandler(dashboard.Deps{
		Auth:                 auth,
		GetStats:             getStats,
		GetSamples:           usecase.NewGetEventSamples(events),
		GetMetadataBreakdown: usecase.NewGetMetadataBreakdown(events),
		Sessions:             dashboardSessions,
		LoginLimiter:         ratelimit.New(cfg.LoginRateLimitPerMinute),
		Panels:               dashboardPanels(pluginManager),
		AdminSessions:        adminSessions,
		ListProjects:         usecase.NewListProjects(projects),
		GetProject:           usecase.NewGetProject(projects),
	})

	adminHandler := admin.NewHandler(admin.Deps{
		Sessions:              adminSessions,
		RegisterAdminUser:     usecase.NewRegisterAdminUser(adminUsers, passwordHasher),
		AuthenticateAdminUser: usecase.NewAuthenticateAdminUser(adminUsers, passwordHasher),
		NeedsSetup:            usecase.NewNeedsAdminSetup(adminUsers),
		CreateProject:         usecase.NewCreateProject(projects, keyHasher),
		ListProjects:          usecase.NewListProjects(projects),
		GetProject:            usecase.NewGetProject(projects),
		DeleteProject:         usecase.NewDeleteProject(projects),
		UpdateProject:         usecase.NewUpdateProject(projects),
		RotateAPIKey:          usecase.NewRotateAPIKey(projects, keyHasher),
		AddAdminUser:          usecase.NewAddAdminUser(adminUsers, passwordHasher),
		ListAdminUsers:        usecase.NewListAdminUsers(adminUsers),
		GetAdminUser:          usecase.NewGetAdminUser(adminUsers),
		DeleteAdminUser:       usecase.NewDeleteAdminUser(adminUsers),
		DashboardSessions:     dashboardSessions,
		LoginLimiter:          ratelimit.New(cfg.LoginRateLimitPerMinute),
	})

	// Each embedded panel's own mux has entries for both its exact root
	// path and everything below it, so both need registering here — a
	// single trailing-slash prefix pattern wouldn't match the bare path.
	mux := http.NewServeMux()
	mux.Handle("/assets/", webui.Handler())
	mux.Handle("/dashboard", dashboardHandler)
	mux.Handle("/dashboard/", dashboardHandler)
	mux.Handle("/admin", adminHandler)
	mux.Handle("/admin/", adminHandler)
	mux.Handle("/", apiRouter)

	slog.Info("traccia listening", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, requestLogger(mux)); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// statusRecorder captures the status code a handler wrote, since
// http.ResponseWriter doesn't expose it after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// loadGeoResolver returns a MaxMind-backed resolver if GEOIP_DB_PATH is
// set and the file opens cleanly, otherwise the no-op default. A bad path
// is a warning, not a fatal error — GeoIP is a nice-to-have, not something
// worth refusing to boot over.
func loadGeoResolver(dbPath string) ports.GeoResolver {
	if dbPath == "" {
		return geoip.NewNoopResolver()
	}
	resolver, err := geoip.NewMaxMindResolver(dbPath)
	if err != nil {
		slog.Warn("could not open GeoIP database, country/city will stay unresolved", "path", dbPath, "error", err)
		return geoip.NewNoopResolver()
	}
	slog.Info("resolving GeoIP", "path", dbPath)
	return resolver
}

func deriveSessionSecret(adminToken string) string {
	sum := sha256.Sum256([]byte("traccia-session-fallback:" + adminToken))
	return hex.EncodeToString(sum[:])
}

func dashboardPanels(manager *plugins.Manager) []dashboard.PanelView {
	panels := manager.Panels()
	views := make([]dashboard.PanelView, 0, len(panels))
	for _, p := range panels {
		views = append(views, dashboard.PanelView{
			Title:     p.Title,
			Kind:      p.Chart,
			EventName: p.EventName,
			GroupBy:   p.GroupBy,
		})
	}
	return views
}
