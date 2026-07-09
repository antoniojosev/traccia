package plugins

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/antoniojosev/traccia/internal/domain"
)

type Manager struct {
	plugins    []*Plugin
	httpClient *http.Client
}

// Load reads every *.js file in dir and loads it as a plugin. A missing
// dir is not an error — plugins are optional. A plugin that fails to load
// is logged and skipped, never fatal: one broken third-party script
// shouldn't stop the whole server from booting.
func Load(dir string, kv KVStore) (*Manager, error) {
	m := &Manager{httpClient: &http.Client{}}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		source, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("plugin: reading file, skipped", "plugin", entry.Name(), "error", err)
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".js")
		plugin, err := loadPlugin(name, string(source), kv, m.httpClient)
		if err != nil {
			slog.Warn("plugin: failed to load, skipped", "plugin", name, "error", err)
			continue
		}

		m.plugins = append(m.plugins, plugin)
		slog.Info("plugin loaded", "plugin", name, "on_event", plugin.onEvent != nil, "panel", plugin.hasPanel)
	}

	return m, nil
}

// Panels returns every dashboard panel declared via registerPanel, in load
// order (alphabetical by filename, since Load walks os.ReadDir's sorted
// output).
func (m *Manager) Panels() []Panel {
	var out []Panel
	for _, p := range m.plugins {
		if p.hasPanel {
			out = append(out, p.panel)
		}
	}
	return out
}

// RunOnEvent runs every loaded plugin's onEvent hook over event, in load
// order. The first plugin to return null/undefined drops the event —
// later plugins don't get a chance to resurrect it. keep is false in that
// case, and event is meaningless.
func (m *Manager) RunOnEvent(event domain.Event) (result domain.Event, keep bool) {
	result = event
	for _, p := range m.plugins {
		result, keep = p.callOnEvent(result)
		if !keep {
			return domain.Event{}, false
		}
	}
	return result, true
}
