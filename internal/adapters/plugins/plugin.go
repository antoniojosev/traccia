package plugins

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/dop251/goja"
)

const defaultTimeout = 100 * time.Millisecond

// Plugin wraps one loaded .js file and its own goja runtime. goja runtimes
// are not safe for concurrent use, so every call into this plugin's JS is
// serialized behind mu — different plugins still run concurrently with
// each other, since each has its own runtime and its own lock.
type Plugin struct {
	name     string
	vm       *goja.Runtime
	mu       sync.Mutex
	timeout  time.Duration
	onEvent  goja.Callable
	hasPanel bool
	panel    Panel
}

func loadPlugin(name, source string, kv KVStore, httpClient *http.Client) (*Plugin, error) {
	vm := goja.New()
	setupSandbox(vm, name, kv, httpClient)

	if _, err := vm.RunString(source); err != nil {
		return nil, fmt.Errorf("running script: %w", err)
	}

	p := &Plugin{name: name, vm: vm, timeout: defaultTimeout}

	if fn, ok := goja.AssertFunction(vm.Get("onEvent")); ok {
		p.onEvent = fn
	}

	if fn, ok := goja.AssertFunction(vm.Get("registerPanel")); ok {
		result, err := fn(goja.Undefined())
		if err != nil {
			return nil, fmt.Errorf("calling registerPanel: %w", err)
		}
		panel, err := parsePanel(result)
		if err != nil {
			return nil, fmt.Errorf("invalid panel from registerPanel: %w", err)
		}
		p.panel = panel
		p.hasPanel = true
	}

	if p.onEvent == nil && !p.hasPanel {
		return nil, fmt.Errorf("defines neither onEvent nor registerPanel — nothing for Traccia to call")
	}

	return p, nil
}

// callOnEvent runs the plugin's onEvent hook with a hard time limit. A
// plugin that errors or times out never breaks ingestion — the event is
// kept unchanged and a warning is logged, because a bug in a third-party
// plugin script is not a reason to start dropping real traffic.
func (p *Plugin) callOnEvent(event domain.Event) (domain.Event, bool) {
	if p.onEvent == nil {
		return event, true
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.vm.ClearInterrupt()

	timer := time.AfterFunc(p.timeout, func() {
		p.vm.Interrupt("plugin exceeded its time budget")
	})
	defer timer.Stop()

	jsEvent := eventToJSValue(p.vm, event)
	result, err := p.onEvent(goja.Undefined(), jsEvent)
	if err != nil {
		log.Printf("[plugin:%s] onEvent error, event kept unchanged: %v", p.name, err)
		return event, true
	}

	return jsValueToEvent(event, result)
}
