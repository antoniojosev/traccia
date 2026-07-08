package plugins

import (
	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/dop251/goja"
)

func eventToJSValue(vm *goja.Runtime, event domain.Event) goja.Value {
	obj := vm.NewObject()
	obj.Set("visitorId", event.VisitorID)
	obj.Set("type", string(event.Type))
	obj.Set("name", event.Name)
	obj.Set("path", event.Path)
	obj.Set("referrer", event.Referrer)
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	obj.Set("metadata", metadata)
	return obj
}

// jsValueToEvent merges a plugin's returned value back onto base. Returning
// null/undefined drops the event (keep=false). Returning anything that
// isn't a plain object is treated as "didn't understand it, keep the
// event unchanged" — a misbehaving plugin should never be able to silently
// destroy data.
func jsValueToEvent(base domain.Event, val goja.Value) (event domain.Event, keep bool) {
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return domain.Event{}, false
	}

	exported, ok := val.Export().(map[string]interface{})
	if !ok {
		return base, true
	}

	updated := base
	if v, ok := exported["name"].(string); ok {
		updated.Name = v
	}
	if v, ok := exported["path"].(string); ok {
		updated.Path = v
	}
	if v, ok := exported["referrer"].(string); ok {
		updated.Referrer = v
	}
	if v, ok := exported["metadata"].(map[string]interface{}); ok {
		updated.Metadata = v
	}
	return updated, true
}
