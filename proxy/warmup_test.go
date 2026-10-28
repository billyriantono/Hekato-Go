package proxy

import (
	"testing"
	"time"
)

func TestWarmupStagger(t *testing.T) {
	if got := warmupStagger(0); got != 0 {
		t.Fatalf("0 accounts: got %v, want 0", got)
	}
	if got := warmupStagger(1); got != 0 {
		t.Fatalf("1 account: got %v, want 0 (no spread needed)", got)
	}
	// 10 accounts over a 60s window → 6s steps
	if got := warmupStagger(10); got != 6*time.Second {
		t.Fatalf("10 accounts: got %v, want 6s", got)
	}
	// Huge pool: floor keeps a visible spread (60s/200 = 300ms → floored to 500ms,
	// so the full batch still completes in ~100s, not tens of minutes).
	if got := warmupStagger(200); got != 500*time.Millisecond {
		t.Fatalf("200 accounts: got %v, want 500ms floor", got)
	}
	// Small pool: capped so 4+ accounts never exceed the 15s cap per slot
	if got := warmupStagger(4); got != 15*time.Second {
		t.Fatalf("4 accounts: got %v, want 15s cap", got)
	}
	// Tiny pool: keep a visible spread
	if got := warmupStagger(3); got < 500*time.Millisecond {
		t.Fatalf("3 accounts: got %v, want >= 500ms", got)
	}
}

func TestJitterN(t *testing.T) {
	if got := jitterN(0); got != 0 {
		t.Fatalf("jitterN(0) = %d, want 0", got)
	}
	for i := 0; i < 100; i++ {
		if got := jitterN(7); got < 0 || got >= 7 {
			t.Fatalf("jitterN(7) out of range: %d", got)
		}
	}
}
