package plugins

import (
	"fmt"

	"github.com/dop251/goja"
)

// Panel is a dashboard panel a plugin declares via registerPanel(). The
// dashboard renders it server-side from this spec — a plugin never ships
// its own frontend JS, which is the whole reason the dashboard is
// server-rendered HTMX instead of a SPA (see the dashboard package).
type Panel struct {
	Title     string
	EventName string
	Chart     string // "line" | "bars" | "table"
	GroupBy   string
}

func parsePanel(val goja.Value) (Panel, error) {
	exported, ok := val.Export().(map[string]interface{})
	if !ok {
		return Panel{}, fmt.Errorf("registerPanel must return an object")
	}

	panel := Panel{Chart: "table"}
	if v, ok := exported["title"].(string); ok {
		panel.Title = v
	}
	if v, ok := exported["eventName"].(string); ok {
		panel.EventName = v
	}
	if v, ok := exported["chart"].(string); ok {
		panel.Chart = v
	}
	if v, ok := exported["groupBy"].(string); ok {
		panel.GroupBy = v
	}
	if panel.Title == "" {
		return Panel{}, fmt.Errorf("registerPanel result must have a non-empty title")
	}
	return panel, nil
}
