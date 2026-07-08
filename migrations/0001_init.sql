CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE projects (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    domain        TEXT NOT NULL DEFAULT '',
    api_key_hash  TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Durable per-visitor identity set via Identify(). Separate from events so
-- traits aren't repeated on every tracked event.
CREATE TABLE visitors (
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    visitor_id  UUID NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    properties  JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, visitor_id)
);

CREATE TABLE events (
    id             BIGSERIAL PRIMARY KEY,
    project_id     UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    visitor_id     UUID NOT NULL,
    event_type     TEXT NOT NULL,
    event_name     TEXT NOT NULL DEFAULT '',
    path           TEXT NOT NULL DEFAULT '',
    referrer       TEXT NOT NULL DEFAULT '',
    ip_anonymized  INET,
    country        TEXT NOT NULL DEFAULT '',
    city           TEXT NOT NULL DEFAULT '',
    device_type    TEXT NOT NULL DEFAULT '',
    browser        TEXT NOT NULL DEFAULT '',
    os             TEXT NOT NULL DEFAULT '',
    metadata       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_events_project_created  ON events (project_id, created_at DESC);
CREATE INDEX idx_events_project_path     ON events (project_id, path);
CREATE INDEX idx_events_project_visitor  ON events (project_id, visitor_id);
CREATE INDEX idx_events_metadata_gin     ON events USING GIN (metadata);
