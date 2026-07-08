// Package ratelimit is a per-IP token bucket held in memory. Shared by the
// ingest endpoints (httpapi, tuned for legitimate traffic volume) and the
// login forms (dashboard, admin — tuned much stricter, since a login
// attempt should never be as frequent as a pageview).
//
// It's single-node — fine for a self-hosted instance, but doesn't
// coordinate across replicas behind a load balancer. That's an accepted
// limitation, not an oversight.
package ratelimit

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Limiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func New(perMinute int) *Limiter {
	rl := &Limiter{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(float64(perMinute) / 60.0),
		burst:    perMinute,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *Limiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rl.rps, rl.burst)}
		rl.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

// cleanupLoop evicts keys that haven't been seen in a while so the map
// doesn't grow unbounded on a long-running instance.
func (rl *Limiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for key, v := range rl.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

// ClientIP trusts X-Forwarded-For only because Traccia is expected to run
// behind a reverse proxy you control (Caddy/nginx/Dokploy) that sets it —
// if you expose the app directly to the internet, this header is spoofable.
func ClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}
