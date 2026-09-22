package proxy

import (
	"hekato-go/config"
	"net/http"
	"sync"
	"time"
)

// keyLimiter enforces per-API-key requests-per-minute and max in-flight limits.
// State is in-memory only; a restart resets the windows.
// ponytail: single mutex + timestamp slices; swap for a token bucket if keys hit thousands of RPM.
type keyLimiter struct {
	mu       sync.Mutex
	windows  map[string][]time.Time // timestamps of requests in the last minute
	inflight map[string]int
}

func newKeyLimiter() *keyLimiter {
	return &keyLimiter{windows: make(map[string][]time.Time), inflight: make(map[string]int)}
}

// Acquire admits a request for the key or returns a 429 authError. On success
// the caller must call Release(entry.ID) when the request finishes.
func (l *keyLimiter) Acquire(entry *config.ApiKeyEntry) *authError {
	if l == nil || entry == nil || (entry.RPMLimit <= 0 && entry.ConcurrencyLimit <= 0) {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.ConcurrencyLimit > 0 && l.inflight[entry.ID] >= int(entry.ConcurrencyLimit) {
		return newAuthError(http.StatusTooManyRequests, "rate_limit_error", "concurrency limit exceeded")
	}
	if entry.RPMLimit > 0 {
		now := time.Now()
		cutoff := now.Add(-time.Minute)
		w := l.windows[entry.ID]
		i := 0
		for i < len(w) && !w[i].After(cutoff) {
			i++
		}
		w = w[i:]
		if len(w) >= int(entry.RPMLimit) {
			l.windows[entry.ID] = w
			return newAuthError(http.StatusTooManyRequests, "rate_limit_error", "requests per minute limit exceeded")
		}
		l.windows[entry.ID] = append(w, now)
	}
	if entry.ConcurrencyLimit > 0 {
		l.inflight[entry.ID]++
	}
	return nil
}

// Release frees an in-flight slot taken by Acquire.
func (l *keyLimiter) Release(id string) {
	if l == nil || id == "" {
		return
	}
	l.mu.Lock()
	if l.inflight[id] > 0 {
		l.inflight[id]--
	}
	if l.inflight[id] == 0 {
		delete(l.inflight, id)
	}
	l.mu.Unlock()
}
