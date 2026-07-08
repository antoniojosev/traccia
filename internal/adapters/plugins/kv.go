// Package plugins is Traccia's extension mechanism: drop a .js file in
// PLUGINS_DIR and it can hook into event processing (onEvent) and declare
// a dashboard panel (registerPanel) — no recompilation, no separate Node
// process. Each plugin runs in its own goja (pure-Go JS) runtime with a
// curated API surface (log/http/kv) and no filesystem/network/require
// access beyond that — the sandbox is the whole point.
package plugins

import "context"

// KVStore is small persistent storage for plugins, namespaced by plugin
// name so one plugin can't read or clobber another's state.
type KVStore interface {
	Get(ctx context.Context, plugin, key string) (value string, ok bool, err error)
	Set(ctx context.Context, plugin, key, value string) error
}
