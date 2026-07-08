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

Create a project at `http://localhost:8080/admin` — the first visit walks
you through creating your own admin account (username + password, not
`ADMIN_TOKEN` — see [Admin panel](#admin-panel) below). Or script it:

```bash
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "My Site", "domain": "example.com"}'
```

Either way you get back an API key you'll only see once — it's only
needed to *read* stats, never to send events.

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

## Admin panel

`/admin` — a human account (username + password), a different and more
privileged trust level than a project's own dashboard login (which only
proves you know *one* project's API key). The **first visit** to `/admin`
walks you through a one-time setup to create that account; after that it's
a normal login, and there's no open registration — the setup form itself
redirects to login once an account exists, so it can't be used to add a
second one.

- **List** every project (name, domain, ID, created date)
- **Create** a new one, with the same one-time API key reveal as the API
- **Jump straight into a project's dashboard** with one click, without
  needing that project's API key — the admin panel mints the dashboard
  session directly, since an admin account already implies more trust
  than any single project's key

This account is unrelated to `ADMIN_TOKEN`, which stays exactly what it
was: the API's machine credential for scripting `POST /api/v1/projects`
(used by `scripts/seed-demo.sh`, CI, whatever you automate). Pasting a
64-character hex token into a browser login form never made much sense as
a *human* login — now it doesn't have to.

## Plugins

Drop a `.js` file in `PLUGINS_DIR` (default `./plugins`) and restart — no
recompiling, no separate process, no Docker image changes. Each plugin
runs in its own embedded [goja](https://github.com/dop251/goja) runtime
(pure-Go JS, no Node) with a curated sandbox: `log`, `http.post`
(fire-and-forget webhooks), and `kv.get`/`kv.set` (small persistent state,
namespaced per plugin). No filesystem, no `require`, no arbitrary network.

Two extension points:

```js
// Runs before every event is saved. Mutate it, or return null to drop it.
function onEvent(event) {
  if (event.type === "error") {
    http.post("https://hooks.slack.com/services/...", { text: "Error: " + event.name });
  }
  return event;
}

// Runs once at load — declares a panel the dashboard renders server-side.
// A plugin never ships its own frontend JS; this is why the dashboard is
// server-rendered HTMX instead of a SPA.
function registerPanel() {
  return { title: "Calculator usage", eventName: "calculator_used", chart: "line" };
}
```

Full API reference, limitations (a ~100ms time budget per `onEvent` call,
sequential execution per plugin, why a plugin error keeps the event
instead of dropping it) and working examples: [`docs/plugins.md`](docs/plugins.md).

## Security model

Three trust levels, each gating something different:

- **`project_id`** (returned on project creation, embedded in the public
  `<script>` tag): identifies which project an event belongs to. It's not a
  secret — anyone who can read your page source can already send it fake
  events, the same tradeoff Google Analytics, Plausible and Umami all make.
  A per-IP rate limit on `/api/v1/track` and `/api/v1/identify` (in-memory,
  single-node — see `RATE_LIMIT_PER_MINUTE`, default 120/min) is the real
  defense here, not secrecy.
- **`api_key`** (shown once on project creation): gates *reading* that one
  project's aggregated stats. It never appears in client-side code.
- **Admin account** (username + password, see [Admin panel](#admin-panel)):
  gates creating/listing *every* project and viewing any of their
  dashboards — the most privileged tier. Separate from `ADMIN_TOKEN`,
  which is the API's own machine credential and never touches the panel.

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
| `PasswordHasher` | bcrypt | — |

```
cmd/api          entrypoint, wiring
internal/
  domain         Event, Visitor, Project, AdminUser, Stats — no external deps
  ports          interfaces the domain depends on
  usecase        TrackEvent, IdentifyVisitor, GetStats, CreateProject, ...
  adapters/
    postgres     default storage
    httpapi      HTTP transport, no business logic
    useragent    default UA parser
    geoip        default (no-op) geo resolver
    apikey       default API key hasher (SHA-256 — high-entropy tokens)
    password     default password hasher (bcrypt — low-entropy human passwords)
    webui        design system shared by dashboard + admin, served at /assets/
    dashboard    embedded HTMX dashboard (templates, static, sessions)
    admin        embedded HTMX admin panel — accounts, projects, jump to dashboard
    session      shared HMAC-signed cookie sessions (dashboard + admin)
    plugins      goja plugin runtime + EventRepository decorator
sdk/js           tracking script — plain <script> tag or npm package,
                 same file, embedded into the Go binary and served at /t.js
migrations       plain SQL, applied by Postgres' docker-entrypoint-initdb.d
                 on first boot (nothing runs automatically after that)
docs/plugins.md  plugin API reference and limitations
plugins-examples reference plugin scripts (not loaded automatically —
                 copy into PLUGINS_DIR to try one)
```

## Roadmap

- MaxMind GeoIP adapter
- Per-key metadata aggregation for custom events (see the Dashboard
  section's scoping note), which would also let `registerPanel`'s
  `groupBy` actually compute something

## License

MIT
