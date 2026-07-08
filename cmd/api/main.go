package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"

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
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.AdminToken == "" {
		log.Println("warning: ADMIN_TOKEN is not set, POST /api/v1/projects is unreachable")
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = deriveSessionSecret(cfg.AdminToken)
		log.Println("warning: SESSION_SECRET is not set, deriving one from ADMIN_TOKEN — set your own for production")
	}

	ctx := context.Background()
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
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
		log.Fatalf("loading plugins from %s: %v", cfg.PluginsDir, err)
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
	dashboardHandler := dashboard.NewHandler(dashboard.Deps{
		Auth:                 auth,
		GetStats:             getStats,
		GetSamples:           usecase.NewGetEventSamples(events),
		GetMetadataBreakdown: usecase.NewGetMetadataBreakdown(events),
		Sessions:             dashboardSessions,
		LoginLimiter:         ratelimit.New(cfg.LoginRateLimitPerMinute),
		Panels:               dashboardPanels(pluginManager),
	})

	adminHandler := admin.NewHandler(admin.Deps{
		Sessions:              session.New(cfg.SessionSecret, "traccia_admin_session", "/admin"),
		RegisterAdminUser:     usecase.NewRegisterAdminUser(adminUsers, passwordHasher),
		AuthenticateAdminUser: usecase.NewAuthenticateAdminUser(adminUsers, passwordHasher),
		NeedsSetup:            usecase.NewNeedsAdminSetup(adminUsers),
		CreateProject:         usecase.NewCreateProject(projects, keyHasher),
		ListProjects:          usecase.NewListProjects(projects),
		GetProject:            usecase.NewGetProject(projects),
		DeleteProject:         usecase.NewDeleteProject(projects),
		AddAdminUser:          usecase.NewAddAdminUser(adminUsers, passwordHasher),
		ListAdminUsers:        usecase.NewListAdminUsers(adminUsers),
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

	log.Printf("traccia listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
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
		log.Printf("warning: could not open GeoIP database at %s: %v — country/city will stay unresolved", dbPath, err)
		return geoip.NewNoopResolver()
	}
	log.Printf("resolving GeoIP from %s", dbPath)
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
