#!/usr/bin/env bash
# Restores a dump produced by scripts/backup.sh into the docker-compose
# Postgres service. Destructive: drops and recreates every object in the
# target database first. Usage: ./scripts/restore.sh ./backups/traccia-....dump
set -euo pipefail

DUMP_FILE="${1:?usage: restore.sh <dump-file>}"
SERVICE="${SERVICE:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-traccia}"
POSTGRES_DB="${POSTGRES_DB:-traccia}"

[ -f "$DUMP_FILE" ] || { echo "no such file: $DUMP_FILE" >&2; exit 1; }

docker compose exec -T "$SERVICE" pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists < "$DUMP_FILE"

echo "restored $DUMP_FILE into $POSTGRES_DB"
