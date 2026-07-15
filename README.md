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

Login attempts are rate-limited per IP (`LOGIN_RATE_LIMIT_PER_MINUTE`,
default 10/min) — much stricter than the ingest limit, since this is the
only thing standing between an API key and someone brute-forcing it.

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
- **Delete** one, with a confirmation step — irreversibly deletes its
  events and visitors too (`ON DELETE CASCADE`)
- **Jump straight into a project's dashboard** with one click, without
  needing that project's API key — the admin panel mints the dashboard
  session directly, since an admin account already implies more trust
  than any single project's key
- **Add more admin accounts** at `/admin/users`, for a teammate — this is
  the one place open registration *is* allowed, since it's gated by
  already having a valid admin session, not by the account being the
  first one

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
  return { title: "Calculator usage", eventName: "calculator_used", chart: "line", groupBy: "from_currency" };
}
```

A panel's `groupBy` is real: the dashboard groups that event name's
occurrences by that metadata key's value (JSONB `metadata->>'key'`) and
renders the counts — e.g. how many `calculator_used` events had
`from_currency: "USD"` vs `"VES"`. Events missing that key are excluded,
not lumped into an empty bucket.

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
  gates creating/listing/deleting *every* project and viewing any of their
  dashboards — the most privileged tier. Separate from `ADMIN_TOKEN`,
  which is the API's own machine credential and never touches the panel.
  The dashboard's project switcher (top bar, next to "Cerrar sesión")
  only appears — and only works — when the request also carries a valid
  admin session; a plain per-project dashboard session (just an `api_key`)
  can never see or reach another project's data through it.

`/dashboard/login`, `/admin/login` and `/admin/setup` are all rate-limited
per IP (`LOGIN_RATE_LIMIT_PER_MINUTE`, default 10/min) — the actual
defense against brute-forcing a password or API key, since none of these
secrets are otherwise throttled.

`GET /healthz` checks Postgres connectivity (`pool.Ping`), not just "the
process is up" — `pgxpool` connects lazily, so without this check the
server could report healthy before Postgres actually finished booting.

## Backups

Everything lives in one Postgres database (`traccia_pgdata`, the volume in
`docker-compose.yml`), so backing up is just `pg_dump` against the
`postgres` service:

```sh
make backup                                 # writes ./backups/traccia-<UTC timestamp>.dump
make restore FILE=./backups/traccia-....dump # destructive — drops and recreates every object first
```

Both wrap `docker compose exec`, so run them from a shell where the stack
is up (`docker compose up -d`). `pg_dump`'s custom format (`-Fc`) is used,
which is compressed and safe to restore into a different Postgres version
than the one that produced it. There's no automated schedule — cron your
own call to `make backup` (or `scripts/backup.sh` directly) if you want
one; this project deliberately runs no background jobs of its own.

## Deploying to production

`docker-compose.prod.yml` is the same stack hardened for a public host:
Postgres stays on the internal network instead of binding `5432` on the
host, the app publishes no port (a reverse proxy routes to it by domain),
the database password comes from the environment, and both containers get
memory limits and healthchecks.

Required environment variables — the stack refuses to start without them:

```bash
ADMIN_TOKEN=        # machine credential for POST /api/v1/projects
SESSION_SECRET=     # signs dashboard/admin login cookies
POSTGRES_PASSWORD=  # database password (embedded into DATABASE_URL)
```

On [Dokploy](https://dokploy.com): create a **Compose** service pointing
at this repo with `docker-compose.prod.yml` as the compose path, set the
three variables in the Environment tab, then add a domain routing to the
`traccia` service on port `8080` (the compose file already joins the
external `dokploy-network` so Traefik can reach it). Migrations run
automatically on the first boot of a fresh database volume — nothing to
execute by hand.

On a plain VPS without Dokploy: remove the `dokploy-network` block and put
your own reverse proxy (Caddy, nginx) in front of the `traccia` service.

## Architecture

Hexagonal: `internal/domain` and `internal/usecase` have no knowledge of
Postgres or HTTP. Everything they depend on is an interface in
`internal/ports`, with a default implementation under `internal/adapters/*`:

| Port | Default adapter | Swap it for |
|---|---|---|
| `EventRepository` | Postgres | ClickHouse, SQLite |
| `UserAgentParser` | small heuristic parser | a full regex-database parser |
| `GeoResolver` | no-op, or MaxMind if `GEOIP_DB_PATH` is set | IP2Location, a paid geo API |
| `APIKeyHasher` | SHA-256 | — |
| `PasswordHasher` | bcrypt | — |

`GeoResolver`'s MaxMind adapter reads a local GeoLite2/GeoIP2 City
`.mmdb` file — no network calls. The database itself isn't bundled (needs
a free MaxMind account and its license doesn't permit redistribution):
download your own and point `GEOIP_DB_PATH` at it. Unset or a bad path
falls back to the no-op resolver with a warning, never a boot failure.

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
    ratelimit    shared per-IP token bucket (ingest limit + stricter login limit)
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

- A way to recover admin access if you lose the password — right now that
  means deleting the row from `admin_users` directly and redoing setup.
  No email/SMTP exists in this project, so a proper "forgot password"
  flow is a bigger addition than it sounds; documented here rather than
  half-built.
- npm publish for `sdk/js` — the package is ready under `sdk/js/`, just
  never pushed to the registry yet.
- A ClickHouse or SQLite `EventRepository` adapter — the port is already
  adapter-agnostic (see [Architecture](#architecture)), Postgres is just
  the only one implemented so far.

## License

MIT
