package proxy

import (
	"hekato-go/config"
	"testing"
)

func TestKeyLimiterRPM(t *testing.T) {
	l := newKeyLimiter()
	e := &config.ApiKeyEntry{ID: "k1", RPMLimit: 2}
	if ae := l.Acquire(e); ae != nil {
		t.Fatalf("1st: %v", ae)
	}
	if ae := l.Acquire(e); ae != nil {
		t.Fatalf("2nd: %v", ae)
	}
	if ae := l.Acquire(e); ae == nil || ae.status != 429 {
		t.Fatalf("3rd should be 429, got %v", ae)
	}
}

func TestKeyLimiterConcurrency(t *testing.T) {
	l := newKeyLimiter()
	e := &config.ApiKeyEntry{ID: "k2", ConcurrencyLimit: 1}
	if ae := l.Acquire(e); ae != nil {
		t.Fatalf("1st: %v", ae)
	}
	if ae := l.Acquire(e); ae == nil {
		t.Fatal("2nd in-flight should be rejected")
	}
	l.Release(e.ID)
	if ae := l.Acquire(e); ae != nil {
		t.Fatalf("after release: %v", ae)
	}
}

func TestKeyLimiterUnlimitedAndNil(t *testing.T) {
	var l *keyLimiter
	if ae := l.Acquire(&config.ApiKeyEntry{ID: "x", RPMLimit: 1}); ae != nil {
		t.Fatal("nil limiter must admit")
	}
	l = newKeyLimiter()
	for i := 0; i < 100; i++ {
		if ae := l.Acquire(&config.ApiKeyEntry{ID: "y"}); ae != nil {
			t.Fatal("zero limits must admit")
		}
	}
}
