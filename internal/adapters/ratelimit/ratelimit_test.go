package ratelimit_test

import (
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/ratelimit"
)

func TestLimiter_AllowsUpToBurstThenBlocks(t *testing.T) {
	rl := ratelimit.New(2) // burst of 2

	if !rl.Allow("1.2.3.4") {
		t.Fatal("expected first request to be allowed")
	}
	if !rl.Allow("1.2.3.4") {
		t.Fatal("expected second request to be allowed (within burst)")
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("expected third request to be denied")
	}
}

func TestLimiter_TracksKeysIndependently(t *testing.T) {
	rl := ratelimit.New(1)

	if !rl.Allow("1.2.3.4") {
		t.Fatal("expected first key's first request to be allowed")
	}
	if !rl.Allow("5.6.7.8") {
		t.Fatal("expected a different key to have its own budget")
	}
}
