package plugins_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type fakeKV struct {
	mu   sync.Mutex
	data map[string]string
}

func newFakeKV() *fakeKV { return &fakeKV{data: map[string]string{}} }

func (k *fakeKV) Get(_ context.Context, plugin, key string) (string, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.data[plugin+"/"+key]
	return v, ok, nil
}

func (k *fakeKV) Set(_ context.Context, plugin, key, value string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.data[plugin+"/"+key] = value
	return nil
}

func writePlugin(t *testing.T, dir, name, source string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".js"), []byte(source), 0o644); err != nil {
		t.Fatalf("writing plugin file: %v", err)
	}
}
