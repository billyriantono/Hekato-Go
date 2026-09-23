package proxy

import (
	"hekato-go/config"
	accountpool "hekato-go/pool"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// TestSharedStateUnderConcurrency hammers every per-request shared structure
// from many goroutines at once (run with -race). It models many different
// users (API keys) hitting the gateway simultaneously: limiter admission and
// release, account selection with affinity, auto-router decisions and
// feedback, metrics and request-log recording, and per-request log metadata.
func TestSharedStateUnderConcurrency(t *testing.T) {
	mustInitConfig(t)
	for _, id := range []string{"c1", "c2", "c3", "c4"} {
		if err := config.AddAccount(config.Account{ID: id, Email: id + "@x", AccessToken: "t", Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("c1", []string{"claude-haiku-4.5", "claude-sonnet-4.5"})
	p.SetModelList("c2", []string{"claude-sonnet-4.5", "claude-opus-4.5"})
	p.SetModelList("c3", []string{"claude-haiku-4.5", "claude-opus-4.5"})
	p.SetModelList("c4", []string{"claude-sonnet-4.5"})
	t.Cleanup(func() {
		for _, id := range []string{"c1", "c2", "c3", "c4"} {
			p.SetModelList(id, nil)
		}
	})

	h := &Handler{
		pool:       p,
		affinity:   newAccountAffinity(),
		limiter:    newKeyLimiter(),
		autoRouter: newAutoRouter(),
		metrics:    newMetricsCollector(),
	}
	cfg := config.DefaultAutoRouteConfig()
	cfg.Explore = 0.2

	const users, perUser = 24, 60
	keys := make([]*config.ApiKeyEntry, users)
	for i := range keys {
		keys[i] = &config.ApiKeyEntry{ID: "k" + string(rune('a'+i%26)) + string(rune('a'+i/26)), RPMLimit: 1000, ConcurrencyLimit: 8}
	}

	var admitted, rejected, picked int64
	var wg sync.WaitGroup
	for u := 0; u < users; u++ {
		wg.Add(1)
		go func(u int) {
			defer wg.Done()
			key := keys[u]
			for i := 0; i < perUser; i++ {
				if ae := h.limiter.Acquire(key); ae != nil {
					atomic.AddInt64(&rejected, 1)
					continue
				}
				atomic.AddInt64(&admitted, 1)

				sig := routeSignals{InputTokens: 100 * (i % 500), Tools: i % 3, Turns: i % 9}
				if d := h.autoRouter.Resolve(p, cfg, sig, nil, "claude"); d != nil {
					h.autoRouter.Record(d.AccountID, d.Model, i%7 != 0, int64(200+i))
				}
				conv := "conv-" + string(rune('A'+u%5))
				if acc := h.pickAccount(conv, "claude-sonnet-4.5", map[string]bool{}, nil); acc != nil {
					atomic.AddInt64(&picked, 1)
					p.RecordSuccess(acc.ID)
					h.metrics.Record("claude", "claude-sonnet-4.5", acc.ID, i%11 != 0, 50, 0.01, int64(300+i))
					r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
					r.Header.Set("User-Agent", "user-"+string(rune('A'+u%26)))
					w := withClientLogMeta(httptest.NewRecorder(), r)
					h.appendRequestLog(h.newLogEntry(w, "claude", "claude-sonnet-4.5", acc.ID, "success", 50, 10, 0.01, 300, 40, 12))
				}
				h.limiter.Release(key.ID)
			}
		}(u)
	}
	wg.Wait()

	if rejected != 0 {
		t.Fatalf("no request should be rejected: each key runs sequentially within its limits (rejected=%d)", rejected)
	}
	if admitted != users*perUser || picked != users*perUser {
		t.Fatalf("admitted=%d picked=%d want %d", admitted, picked, users*perUser)
	}
	// Every in-flight slot must have been released.
	for _, k := range keys {
		if ae := h.limiter.Acquire(&config.ApiKeyEntry{ID: k.ID, ConcurrencyLimit: 1}); ae != nil {
			t.Fatalf("key %s still holds in-flight slots after all releases", k.ID)
		}
		h.limiter.Release(k.ID)
	}
	_, totals, _, _, _ := h.metrics.Query(60, 1)
	if totals.Requests != users*perUser {
		t.Fatalf("metrics lost requests under concurrency: %d want %d", totals.Requests, users*perUser)
	}
	if got := len(h.getRequestLogs()); got != requestLogsMaxSize {
		t.Fatalf("request log ring should be full (%d), got %d", requestLogsMaxSize, got)
	}
	dec, cands := h.autoRouter.Snapshot()
	if len(dec) != decisionsRingSize || len(cands) == 0 {
		t.Fatalf("router state inconsistent: %d decisions, %d candidates", len(dec), len(cands))
	}
}

// TestLimiterConcurrencyCapIsExact verifies the max-concurrent limit never
// over-admits when many goroutines race for the same key.
func TestLimiterConcurrencyCapIsExact(t *testing.T) {
	l := newKeyLimiter()
	key := &config.ApiKeyEntry{ID: "shared", ConcurrencyLimit: 5}
	var inflight, maxSeen int64
	var wg sync.WaitGroup
	for g := 0; g < 64; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if l.Acquire(key) != nil {
					continue
				}
				n := atomic.AddInt64(&inflight, 1)
				for {
					m := atomic.LoadInt64(&maxSeen)
					if n <= m || atomic.CompareAndSwapInt64(&maxSeen, m, n) {
						break
					}
				}
				atomic.AddInt64(&inflight, -1)
				l.Release(key.ID)
			}
		}()
	}
	wg.Wait()
	if maxSeen > 5 {
		t.Fatalf("concurrency cap breached: saw %d in flight, limit 5", maxSeen)
	}
	if l.Acquire(&config.ApiKeyEntry{ID: "shared", ConcurrencyLimit: 1}) != nil {
		t.Fatal("slots leaked: key still at capacity after all releases")
	}
}
