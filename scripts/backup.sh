#!/usr/bin/env bash
# Dumps the docker-compose Postgres service to a local .dump file (pg_dump's
# custom format, restorable with scripts/restore.sh). Run from the repo
# root while the stack is up: `./scripts/backup.sh`.
set -euo pipefail

SERVICE="${SERVICE:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-traccia}"
POSTGRES_DB="${POSTGRES_DB:-traccia}"
OUT_DIR="${OUT_DIR:-./backups}"
OUT_FILE="$OUT_DIR/traccia-$(date -u +%Y%m%dT%H%M%SZ).dump"

mkdir -p "$OUT_DIR"

docker compose exec -T "$SERVICE" pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB" > "$OUT_FILE"

echo "wrote $OUT_FILE"
