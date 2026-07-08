# Plugins

Traccia's extension mechanism: drop a `.js` file in `PLUGINS_DIR` (default
`./plugins`, relative to the working directory the binary runs from) and
restart. No compiling, no separate process, no Docker image changes.

Each plugin runs in its own [goja](https://github.com/dop251/goja) runtime
— pure-Go JavaScript, no Node, no V8. A plugin can define either or both of
two functions:

## `onEvent(event)`

Called once per event, right before it's saved — for every pageview,
custom event, and error. Return the event (optionally mutated) to keep it,
or `null`/`undefined` to drop it entirely (it's never stored).

```js
function onEvent(event) {
  // event: { visitorId, type, name, path, referrer, metadata }
  if (event.type === "error") {
    http.post("https://hooks.slack.com/services/...", {
      text: `Error on ${event.path}: ${event.name}`
    });
  }
  return event;
}
```

Mutating `event.metadata` (or `.path`, `.name`, `.referrer`) before
returning changes what gets stored:

```js
function onEvent(event) {
  event.metadata.enriched_at = Date.now();
  return event;
}
```

## `registerPanel()`

Called once at load time. Return a panel spec and the dashboard renders it
on the overview page — you write JS that computes/describes data, never
frontend code:

```js
function registerPanel() {
  return {
    title: "Calculator usage",
    eventName: "calculator_used",
    chart: "line",   // "line" | "bars" | "table"
    groupBy: "amount"
  };
}
```

A plugin missing a `title` is rejected (logged, not loaded) — everything
else in the spec is optional.

## Sandbox API

This is the entire surface a plugin can touch. No filesystem, no
`require`/`import`, no arbitrary network access — the sandbox is the
actual security boundary, not a suggestion.

| Global | Signature | Notes |
|---|---|---|
| `log.info(msg)` / `log.warn(msg)` / `log.error(msg)` | `(string) => void` | Written to the server's log, prefixed `[plugin:<name>]` |
| `http.post(url, body)` | `(string, object) => void` | Fire-and-forget, 5s timeout, `body` is JSON-serialized. No response is exposed to the plugin. |
| `kv.get(key)` | `(string) => string \| undefined` | Small persistent state, namespaced per plugin — one plugin can't read another's |
| `kv.set(key, value)` | `(string, string) => void` | Values are strings; encode your own JSON if you need structure |

## Limitations (by design, not yet — read this before relying on something)

- **~100ms time budget per `onEvent` call.** A plugin that hangs (infinite
  loop, etc.) gets interrupted and the event is kept unchanged — a bug in
  a plugin script is never a reason to start dropping real traffic. This
  isn't configurable per-plugin yet.
- **Plugins run sequentially, not in parallel, within themselves.** Each
  plugin has its own goja runtime guarded by a mutex (goja runtimes aren't
  safe for concurrent use), so concurrent requests hitting the same plugin
  queue up behind that lock. Different plugins run independently of each
  other. Don't expect high-throughput ingest with plugins that do
  meaningful work per event.
- **A plugin error keeps the event unchanged, it never drops it.** Only an
  explicit `return null`/`return undefined` drops an event. This is
  deliberate: a crashing plugin shouldn't silently start losing data.
- **`registerPanel()` runs once, at load time** — it can't return different
  panels based on runtime state.
- **No per-metadata-key aggregation in the dashboard yet.** A panel's
  `groupBy` is captured in the spec but the dashboard doesn't compute
  anything from it yet — see the main README's Dashboard section.
- **A broken plugin (syntax error, or one that defines neither `onEvent`
  nor `registerPanel`) is skipped with a log line, not fatal** — one bad
  script never stops the server from booting.

## Examples

See `plugins-examples/` for working scripts:
- `slack-error-alert.js` — posts to a webhook whenever an `error` event comes in
- `panel-example.js` — declares a dashboard panel

Copy either into your `PLUGINS_DIR` (real plugin directories are
gitignored — `plugins-examples/` is just reference code, not something
that loads automatically) and restart to try it.
