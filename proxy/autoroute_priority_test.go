package proxy

import (
	"hekato-go/config"
	accountpool "hekato-go/pool"
	"testing"
)

// A weight-2+ account wins its tier while it is not slow and has a free slot,
// even when the bandit would score the other account higher.
func TestAutoRouterPrefersIdlePriorityAccount(t *testing.T) {
	mustInitConfig(t)
	if err := config.AddAccount(config.Account{ID: "p-normal", Email: "n@x", AccessToken: "t", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := config.AddAccount(config.Account{ID: "p-friend", Email: "f@x", AccessToken: "t", Enabled: true, Weight: 2, MaxConcurrency: 1}); err != nil {
		t.Fatal(err)
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("p-normal", []string{"claude-sonnet-4.5"})
	p.SetModelList("p-friend", []string{"claude-sonnet-4.6"})
	t.Cleanup(func() { p.SetModelList("p-normal", nil); p.SetModelList("p-friend", nil) })

	r := newAutoRouter()
	cfg := config.DefaultAutoRouteConfig()
	cfg.Explore = 0
	// Make the friend's pair look far worse to the bandit.
	for i := 0; i < 50; i++ {
		r.Record("p-normal", "claude-sonnet-4.5", true, 500)
		r.Record("p-friend", "claude-sonnet-4.6", true, 20000)
	}
	sig := routeSignals{InputTokens: 500, Tools: 3, Reasoning: true} // balanced

	if d := r.Resolve(p, cfg, sig, nil, "claude"); d == nil || d.AccountID != "p-friend" {
		t.Fatalf("idle priority account should win, got %+v", d)
	}
	friend := p.GetByID("p-friend")
	if !r.priorityWaiting(p, cfg, sig, nil, p.GetByID("p-normal")) {
		t.Fatal("pin on normal account should yield to idle priority account")
	}

	release, err := acquireAccountSlot(friend)
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Resolve(p, cfg, sig, nil, "claude"); d == nil || d.AccountID != "p-normal" {
		t.Fatalf("saturated priority account should overflow, got %+v", d)
	}
	release()

	r.RecordSlow("p-friend", "claude-sonnet-4.6", "test")
	if d := r.Resolve(p, cfg, sig, nil, "claude"); d == nil || d.AccountID != "p-normal" {
		t.Fatalf("slow priority account should be skipped, got %+v", d)
	}
}
