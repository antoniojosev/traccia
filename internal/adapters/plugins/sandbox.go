package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/dop251/goja"
)

const httpPostTimeout = 5 * time.Second

// setupSandbox exposes exactly log/http/kv to a plugin's runtime — no
// filesystem, no require/import, no raw network beyond http.post. This is
// the actual security boundary: a plugin can only do what these globals
// let it do.
func setupSandbox(vm *goja.Runtime, name string, kv KVStore, httpClient *http.Client) {
	logObj := vm.NewObject()
	logObj.Set("info", func(call goja.FunctionCall) goja.Value {
		log.Printf("[plugin:%s] INFO %s", name, call.Argument(0).String())
		return goja.Undefined()
	})
	logObj.Set("warn", func(call goja.FunctionCall) goja.Value {
		log.Printf("[plugin:%s] WARN %s", name, call.Argument(0).String())
		return goja.Undefined()
	})
	logObj.Set("error", func(call goja.FunctionCall) goja.Value {
		log.Printf("[plugin:%s] ERROR %s", name, call.Argument(0).String())
		return goja.Undefined()
	})
	vm.Set("log", logObj)

	httpObj := vm.NewObject()
	httpObj.Set("post", func(call goja.FunctionCall) goja.Value {
		url := call.Argument(0).String()
		var payload []byte
		if len(call.Arguments) > 1 {
			payload, _ = json.Marshal(call.Argument(1).Export())
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), httpPostTimeout)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
			if err != nil {
				log.Printf("[plugin:%s] http.post: building request: %v", name, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := httpClient.Do(req)
			if err != nil {
				log.Printf("[plugin:%s] http.post: %v", name, err)
				return
			}
			resp.Body.Close()
		}()
		return goja.Undefined()
	})
	vm.Set("http", httpObj)

	kvObj := vm.NewObject()
	kvObj.Set("get", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		value, ok, err := kv.Get(context.Background(), name, key)
		if err != nil {
			log.Printf("[plugin:%s] kv.get error: %v", name, err)
			return goja.Undefined()
		}
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(value)
	})
	kvObj.Set("set", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		value := call.Argument(1).String()
		if err := kv.Set(context.Background(), name, key, value); err != nil {
			log.Printf("[plugin:%s] kv.set error: %v", name, err)
		}
		return goja.Undefined()
	})
	vm.Set("kv", kvObj)
}
