package modelsdev

import (
	"hekato-go/config"
	"path/filepath"
	"testing"
	"time"
)

func TestListPricesOpenCode(t *testing.T) {
	models, _, err := List()
	if err != nil {
		t.Skip("models.dev unreachable:", err)
	}
	var free, paid int
	for _, m := range models {
		if m.Provider != "opencode" {
			continue
		}
		if m.Free {
			free++
		} else {
			paid++
		}
	}
	if free == 0 || paid == 0 {
		t.Fatalf("expected both free and paid opencode models, got free=%d paid=%d", free, paid)
	}
	m, ok := Find("opencode", "gpt-5.6-sol")
	if !ok || m.Input <= 0 || m.ContextLimit == 0 {
		t.Fatalf("Find(gpt-5.6-sol) = %+v ok=%v", m, ok)
	}
}

func TestCatalogPersistsAndReloads(t *testing.T) {
	if err := config.Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatal(err)
	}
	// Refresh (not List) so the pull happens with the store in place — an
	// earlier test may already have warmed the in-memory cache.
	if _, _, err := Refresh(); err != nil {
		t.Skip("models.dev unreachable:", err)
	}
	// Drop the in-memory cache: a fresh process must serve from the blob store
	// without touching the network.
	mu.Lock()
	cache, fetchedAt, blobLoaded = nil, time.Time{}, false
	mu.Unlock()

	// Reading the blob directly proves the catalog survived, without letting a
	// silent network refetch fake the result.
	mu.Lock()
	loadBlobLocked()
	n, at := len(cache), fetchedAt
	mu.Unlock()
	if n == 0 || at.IsZero() {
		t.Fatalf("restore from blob failed: n=%d at=%v", n, at)
	}
}

// The dotted and dashed spellings of one model can carry different windows;
// ContextLimit must report the smaller, or routing over-estimates and overflows.
func TestContextLimitTakesSmallestSpelling(t *testing.T) {
	if ContextLimit("claude-opus-4-6") == 0 {
		t.Skip("models.dev catalog unavailable")
	}
	dotted, dashed := ContextLimit("claude-opus-4.6"), ContextLimit("claude-opus-4-6")
	if dotted != dashed {
		t.Errorf("claude-opus-4.6=%d but claude-opus-4-6=%d; want the smaller for both", dotted, dashed)
	}
	if n := ContextLimit("no-such-model-anywhere"); n != 0 {
		t.Errorf("unknown model should report 0, got %d", n)
	}
}
