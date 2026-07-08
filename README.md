# Traccia

Self-hosted, privacy-first web analytics. One Go binary, one Postgres
database, no external dependencies to track pageviews, custom events and
errors across as many projects as you want.

Built to be extended: swap storage, GeoIP resolution or user-agent parsing
by implementing a small Go interface — no forking required.

## Why

Most self-hosted analytics tools are either a full product you can't extend
without forking (Umami, GoatCounter) or a library you have to wire into your
own backend (no dashboard, no story for non-Go projects). Traccia aims to be
both: a deployable product with sane defaults, and a set of ports you can
swap without touching a usecase.

## Quickstart

```bash
cp .env.example .env   # set ADMIN_TOKEN to a long random string
docker compose up -d
```

Create a project (returns an API key you'll only see once — that key is
only needed to *read* stats, not to send events):

```bash
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "My Site", "domain": "example.com"}'
```

Embed the tracking script on your site, using the `project_id` from the
response above (this ID is public by design — see [Security model](#security-model)):

```html
<script src="http://localhost:8080/t.js" data-project="<project_id>" defer></script>
```

Pageviews are tracked automatically (including SPA route changes). For
custom events:

```js
traccia.track("calculator_used", { from_currency: "USD", to_currency: "VES" });
traccia.identify({ plan: "pro" }); // durable traits, not repeated per event
```

Or declaratively, without writing JS:

```html
<button data-traccia-event="signup_clicked">Sign up</button>
```

Read stats (requires the secret API key from project creation). By default,
bot traffic (detected via user-agent) is excluded and so is anyone you've
`identify()`'d with a `name`:

```bash
curl "http://localhost:8080/api/v1/stats?since=2026-07-01T00:00:00Z" \
  -H "Authorization: Bearer <api_key>"

# include bots, exclude anyone you've named (e.g. yourself)
curl "http://localhost:8080/api/v1/stats?include_bots=true&exclude_named=true" \
  -H "Authorization: Bearer <api_key>"
```

Or skip curl entirely: visit `http://localhost:8080/dashboard` and log in
with the API key — see [Dashboard](#dashboard) below.

## Dashboard

Server-rendered (HTMX + Go templates, no frontend build step) and embedded
in the binary via `embed.FS` — same "one binary" story as the tracking
script. Log in with a project's API key at `/dashboard` to see:

- Total events / unique visitors and a visits-over-time chart (uPlot,
  vendored, no CDN)
- Top pages, referrers, device types, browsers, operating systems
- Custom events and errors, grouped by name, with a drill-down into the
  last 50 occurrences of any one of them (including full metadata)
- Toggles for the same `exclude_named`/`include_bots` filters the API
  supports

Sessions are signed cookies (`SESSION_SECRET`, falls back to one derived
from `ADMIN_TOKEN` if unset — set your own for production) — there's no
server-side session store, so nothing to clean up and nothing lost on
restart except active logins.

Want to see it with data instead of an empty state? `EVENTS=300 ADMIN_TOKEN=... ./scripts/seed-demo.sh`
creates a demo project and floods it with a realistic mix of traffic.

**Scoping note**: per-key metadata aggregation (e.g. "average `amount` for
`calculator_used`") isn't implemented — the drill-down shows raw recent
events with their metadata instead. Generic JSONB key aggregation is a
sharper SQL problem than it looks and didn't seem worth the risk without
being able to verify it against a real database in this environment (see
the hardening PR). Documented here rather than silently scoped out.

## Security model

Two kinds of keys, two trust levels:

- **`project_id`** (returned on project creation, embedded in the public
  `<script>` tag): identifies which project an event belongs to. It's not a
  secret — anyone who can read your page source can already send it fake
  events, the same tradeoff Google Analytics, Plausible and Umami all make.
  A per-IP rate limit on `/api/v1/track` and `/api/v1/identify` (in-memory,
  single-node — see `RATE_LIMIT_PER_MINUTE`, default 120/min) is the real
  defense here, not secrecy.
- **`api_key`** (shown once on project creation): the only thing gated by
  it is *reading* aggregated stats. It never appears in client-side code.

## Architecture

Hexagonal: `internal/domain` and `internal/usecase` have no knowledge of
Postgres or HTTP. Everything they depend on is an interface in
`internal/ports`, with a default implementation under `internal/adapters/*`:

| Port | Default adapter | Swap it for |
|---|---|---|
| `EventRepository` | Postgres | ClickHouse, SQLite |
| `UserAgentParser` | small heuristic parser | a full regex-database parser |
| `GeoResolver` | no-op | MaxMind/IP2Location |
| `APIKeyHasher` | SHA-256 | — |

```
cmd/api          entrypoint, wiring
internal/
  domain         Event, Visitor, Project, Stats — no external deps
  ports          interfaces the domain depends on
  usecase        TrackEvent, IdentifyVisitor, GetStats, CreateProject
  adapters/
    postgres     default storage
    httpapi      HTTP transport, no business logic
    useragent    default UA parser
    geoip        default (no-op) geo resolver
    apikey       default key hasher
    dashboard    embedded HTMX dashboard (templates, static, sessions)
sdk/js           tracking script — plain <script> tag or npm package,
                 same file, embedded into the Go binary and served at /t.js
migrations       plain SQL, applied by Postgres' docker-entrypoint-initdb.d
                 on first boot (nothing runs automatically after that)
```

## Roadmap

- Plugin runtime via an embedded JS interpreter (`goja`) — extension points
  like `onEvent`/`registerPanel`, so custom logic (and custom dashboard
  panels) ship as a `.js` file dropped in `plugins/`, no recompilation
- MaxMind GeoIP adapter
- Per-key metadata aggregation for custom events (see the Dashboard
  section's scoping note)

## License

MIT
