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
	"github.com/antoniojosev/traccia/internal/adapters/plugins"
	"github.com/antoniojosev/traccia/internal/adapters/postgres"
	"github.com/antoniojosev/traccia/internal/adapters/session"
	"github.com/antoniojosev/traccia/internal/adapters/useragent"
	"github.com/antoniojosev/traccia/internal/config"
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
	geoResolver := geoip.NewNoopResolver()
	keyHasher := apikey.NewSHA256Hasher()
	pluginKV := postgres.NewPluginKVRepository(pool)

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
		RateLimiter:     httpapi.NewRateLimiter(cfg.RateLimitPerMinute),
	})

	dashboardSessions := session.New(cfg.SessionSecret, "traccia_session", "/dashboard")
	dashboardHandler := dashboard.NewHandler(dashboard.Deps{
		Auth:       auth,
		GetStats:   getStats,
		GetSamples: usecase.NewGetEventSamples(events),
		Sessions:   dashboardSessions,
		Panels:     dashboardPanels(pluginManager),
	})

	adminHandler := admin.NewHandler(admin.Deps{
		AdminToken:        cfg.AdminToken,
		Sessions:          session.New(cfg.SessionSecret, "traccia_admin_session", "/admin"),
		CreateProject:     usecase.NewCreateProject(projects, keyHasher),
		ListProjects:      usecase.NewListProjects(projects),
		GetProject:        usecase.NewGetProject(projects),
		DashboardSessions: dashboardSessions,
	})

	// Each embedded panel's own mux has entries for both its exact root
	// path and everything below it, so both need registering here — a
	// single trailing-slash prefix pattern wouldn't match the bare path.
	mux := http.NewServeMux()
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

func deriveSessionSecret(adminToken string) string {
	sum := sha256.Sum256([]byte("traccia-session-fallback:" + adminToken))
	return hex.EncodeToString(sum[:])
}

func dashboardPanels(manager *plugins.Manager) []dashboard.PanelView {
	panels := manager.Panels()
	views := make([]dashboard.PanelView, 0, len(panels))
	for _, p := range panels {
		views = append(views, dashboard.PanelView{Title: p.Title, Kind: p.Chart})
	}
	return views
}
