package main

import (
	"context"
	"log"
	"net/http"

	"github.com/antoniojosev/traccia/internal/adapters/apikey"
	"github.com/antoniojosev/traccia/internal/adapters/geoip"
	"github.com/antoniojosev/traccia/internal/adapters/httpapi"
	"github.com/antoniojosev/traccia/internal/adapters/postgres"
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

	ctx := context.Background()
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer pool.Close()

	// Default adapters — swap any of these for your own implementation of
	// the matching port without touching a usecase.
	events := postgres.NewEventRepository(pool)
	projects := postgres.NewProjectRepository(pool)
	visitors := postgres.NewVisitorRepository(pool)
	uaParser := useragent.NewHeuristicParser()
	geoResolver := geoip.NewNoopResolver()
	keyHasher := apikey.NewSHA256Hasher()

	router := httpapi.NewRouter(httpapi.Deps{
		AdminToken:      cfg.AdminToken,
		Auth:            usecase.NewAuthenticateProject(projects, keyHasher),
		CreateProject:   usecase.NewCreateProject(projects, keyHasher),
		TrackEvent:      usecase.NewTrackEvent(events, uaParser, geoResolver),
		IdentifyVisitor: usecase.NewIdentifyVisitor(visitors),
		GetStats:        usecase.NewGetStats(events),
		RateLimiter:     httpapi.NewRateLimiter(cfg.RateLimitPerMinute),
	})

	log.Printf("traccia listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}
