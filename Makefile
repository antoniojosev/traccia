.PHONY: test test-integration smoke

test:
	go test ./...

# Starts a throwaway Postgres on :5433, runs the integration test suite
# against it, tears the container down. Safe to re-run: a leftover
# container from a crashed run gets replaced, not duplicated.
test-integration:
	@docker rm -f traccia-test-postgres >/dev/null 2>&1 || true
	docker run -d --name traccia-test-postgres \
		-e POSTGRES_USER=traccia -e POSTGRES_PASSWORD=traccia -e POSTGRES_DB=traccia_test \
		-p 5433:5432 postgres:16-alpine >/dev/null
	@echo "waiting for postgres..."
	@until docker exec traccia-test-postgres pg_isready -U traccia >/dev/null 2>&1; do sleep 1; done
	TRACCIA_TEST_DATABASE_URL=postgres://traccia:traccia@localhost:5433/traccia_test?sslmode=disable \
		go test -tags=integration ./...; \
		status=$$?; \
		docker rm -f traccia-test-postgres >/dev/null 2>&1; \
		exit $$status

# Full end-to-end check against a real docker-compose stack. See
# scripts/smoke.sh for what it exercises. docker-compose reads .env on its
# own; smoke.sh needs it exported into its own shell too.
smoke:
	docker compose up -d --build
	set -a && [ -f .env ] && . ./.env; set +a; \
		./scripts/smoke.sh; status=$$?; \
		docker compose down -v; \
		exit $$status
