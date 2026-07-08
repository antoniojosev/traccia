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

Read stats (requires the secret API key from project creation):

```bash
curl "http://localhost:8080/api/v1/stats?since=2026-07-01T00:00:00Z" \
  -H "Authorization: Bearer <api_key>"
```

## Security model

Two kinds of keys, two trust levels:

- **`project_id`** (returned on project creation, embedded in the public
  `<script>` tag): identifies which project an event belongs to. It's not a
  secret — anyone who can read your page source can already send it fake
  events, the same tradeoff Google Analytics, Plausible and Umami all make.
  Rate limiting/bot filtering is the real defense here, not secrecy (not yet
  implemented — see [Roadmap](#roadmap)).
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
sdk/js           tracking script — plain <script> tag or npm package,
                 same file, embedded into the Go binary and served at /t.js
migrations       plain SQL, applied by Postgres' docker-entrypoint-initdb.d
                 on first boot (nothing runs automatically after that)
```

## Roadmap

- Dashboard UI (embedded in the binary via `embed.FS`, same "one binary"
  story as the tracking script)
- Plugin runtime via an embedded JS interpreter (`goja`) — extension points
  like `onEvent`/`beforeStore`/`onStatsQuery`, so custom logic ships as a
  `.js` file dropped in `plugins/`, no recompilation
- Rate limiting on `/api/v1/track`
- MaxMind GeoIP adapter

## License

MIT
