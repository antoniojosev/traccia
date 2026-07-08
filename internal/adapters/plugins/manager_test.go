package plugins_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/antoniojosev/traccia/internal/adapters/plugins"
	"github.com/antoniojosev/traccia/internal/domain"
)

func TestLoad_MissingDirIsNotAnError(t *testing.T) {
	m, err := plugins.Load(t.TempDir()+"/does-not-exist", newFakeKV())
	if err != nil {
		t.Fatalf("expected no error for a missing plugins dir, got %v", err)
	}
	if len(m.Panels()) != 0 {
		t.Errorf("expected no panels from an empty manager")
	}
}

func TestLoad_SkipsPluginsWithNoRecognizedHook(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "useless", `function notAHook() { return 1; }`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := domain.Event{Path: "/"}
	result, keep := m.RunOnEvent(event)
	if !keep || result.Path != "/" {
		t.Errorf("expected the event to pass through unchanged since no plugin loaded, got %+v keep=%v", result, keep)
	}
}

func TestLoad_SkipsPluginWithSyntaxErrorButLoadsOthers(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "broken", `function onEvent(e) { this is not valid javascript`)
	writePlugin(t, dir, "good", `function onEvent(e) { e.name = "tagged"; return e; }`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("Load itself should not fail because one plugin is broken: %v", err)
	}

	result, keep := m.RunOnEvent(domain.Event{Type: domain.EventTypeCustom, Name: "original"})
	if !keep {
		t.Fatal("expected event to be kept")
	}
	if result.Name != "tagged" {
		t.Errorf("expected the good plugin to still run, got name=%q", result.Name)
	}
}

func TestRunOnEvent_MutatesMetadata(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "enrich", `
		function onEvent(event) {
			event.metadata.enriched = true;
			return event;
		}
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, keep := m.RunOnEvent(domain.Event{Metadata: map[string]any{"amount": 100}})
	if !keep {
		t.Fatal("expected event to be kept")
	}
	if result.Metadata["enriched"] != true {
		t.Errorf("expected metadata.enriched=true, got %+v", result.Metadata)
	}
	// A metadata key the plugin never touches keeps its original Go value
	// as-is (goja's map wrapping only converts what JS actually reads or
	// writes) — so this is the original int, not a JS-round-tripped number.
	if fmt.Sprint(result.Metadata["amount"]) != "100" {
		t.Errorf("expected original metadata to survive the round trip, got %+v", result.Metadata)
	}
}

func TestRunOnEvent_DropsEventWhenPluginReturnsNull(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "filter", `
		function onEvent(event) {
			if (event.type === "error") { return null; }
			return event;
		}
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, keep := m.RunOnEvent(domain.Event{Type: domain.EventTypeError})
	if keep {
		t.Error("expected the error event to be dropped")
	}

	_, keep = m.RunOnEvent(domain.Event{Type: domain.EventTypePageview})
	if !keep {
		t.Error("expected the pageview event to survive")
	}
}

func TestRunOnEvent_PluginErrorKeepsEventUnchanged(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "crashy", `
		function onEvent(event) {
			throw new Error("boom");
		}
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, keep := m.RunOnEvent(domain.Event{Path: "/original"})
	if !keep {
		t.Fatal("a plugin error must not drop the event")
	}
	if result.Path != "/original" {
		t.Errorf("expected the event to be unchanged after a plugin error, got %+v", result)
	}
}

func TestRunOnEvent_ChainsMultiplePluginsInLoadOrder(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "a_first", `
		function onEvent(event) { event.metadata.steps = (event.metadata.steps || "") + "a"; return event; }
	`)
	writePlugin(t, dir, "b_second", `
		function onEvent(event) { event.metadata.steps = (event.metadata.steps || "") + "b"; return event; }
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, keep := m.RunOnEvent(domain.Event{Metadata: map[string]any{}})
	if !keep {
		t.Fatal("expected event to be kept")
	}
	if result.Metadata["steps"] != "ab" {
		t.Errorf("expected plugins to run in filename order (a then b), got %q", result.Metadata["steps"])
	}
}

func TestPlugin_TimeoutInterruptsInfiniteLoop(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "hangs", `
		function onEvent(event) {
			while (true) {}
			return event;
		}
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		m.RunOnEvent(domain.Event{Path: "/"})
		close(done)
	}()

	select {
	case <-done:
		// good — the timeout interrupted the infinite loop
	case <-time.After(2 * time.Second):
		t.Fatal("RunOnEvent did not return — the plugin timeout did not interrupt an infinite loop")
	}
}

func TestPlugin_TimeoutDoesNotPermanentlyBreakTheRuntime(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "sometimes_hangs", `
		var calls = 0;
		function onEvent(event) {
			calls++;
			if (calls === 1) { while (true) {} }
			event.name = "call-" + calls;
			return event;
		}
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		m.RunOnEvent(domain.Event{})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("first (hanging) call never returned")
	}

	result, keep := m.RunOnEvent(domain.Event{})
	if !keep {
		t.Fatal("expected second call to succeed")
	}
	if result.Name != "call-2" {
		t.Errorf("expected the runtime to be reusable after a timeout, got name=%q", result.Name)
	}
}

func TestManager_Panels(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "chart", `
		function registerPanel() {
			return { title: "Calculator usage", eventName: "calculator_used", chart: "line", groupBy: "amount" };
		}
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	panels := m.Panels()
	if len(panels) != 1 {
		t.Fatalf("expected 1 panel, got %d", len(panels))
	}
	if panels[0].Title != "Calculator usage" || panels[0].Chart != "line" {
		t.Errorf("unexpected panel: %+v", panels[0])
	}
}

func TestLoad_RejectsPanelWithoutTitle(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "bad_panel", `
		function registerPanel() { return { chart: "line" }; }
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("Load itself should not fail: %v", err)
	}
	if len(m.Panels()) != 0 {
		t.Error("expected the titleless panel to be rejected, not registered")
	}
}

func TestPlugin_KVRoundTripsAcrossInvocations(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "counter", `
		function onEvent(event) {
			var count = parseInt(kv.get("count") || "0", 10);
			count++;
			kv.set("count", String(count));
			event.metadata.count = count;
			return event;
		}
	`)

	kv := newFakeKV()
	m, err := plugins.Load(dir, kv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	first, _ := m.RunOnEvent(domain.Event{Metadata: map[string]any{}})
	second, _ := m.RunOnEvent(domain.Event{Metadata: map[string]any{}})

	if first.Metadata["count"] != int64(1) {
		t.Errorf("expected first call count=1, got %+v", first.Metadata["count"])
	}
	if second.Metadata["count"] != int64(2) {
		t.Errorf("expected kv state to persist across calls, got %+v", second.Metadata["count"])
	}
}

func TestPlugin_HTTPPostReachesTarget(t *testing.T) {
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- string(body)
	}))
	defer server.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "notifier", `
		function onEvent(event) {
			if (event.type === "error") {
				http.post(`+"`"+server.URL+"`"+`, { message: "something broke" });
			}
			return event;
		}
	`)

	m, err := plugins.Load(dir, newFakeKV())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m.RunOnEvent(domain.Event{Type: domain.EventTypeError})

	select {
	case body := <-received:
		if body == "" {
			t.Error("expected a non-empty webhook body")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("http.post never reached the test server")
	}
}
