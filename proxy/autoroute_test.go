package proxy

import (
	"errors"
	"hekato-go/config"
	accountpool "hekato-go/pool"
	"net/http/httptest"
	"testing"
)

func TestClassifyTier(t *testing.T) {
	if classifyTier(routeSignals{InputTokens: 500}) != 0 {
		t.Fatal("short prompt should be fast")
	}
	if classifyTier(routeSignals{InputTokens: 500, Tools: 3}) != 1 {
		t.Fatal("tools should be balanced")
	}
	if classifyTier(routeSignals{InputTokens: 50000}) != 2 || classifyTier(routeSignals{Thinking: true}) != 2 {
		t.Fatal("huge context / thinking should be strong")
	}
}

func TestAutoRouterPicksTierAndLearns(t *testing.T) {
	mustInitConfig(t)
	for _, id := range []string{"a1", "a2"} {
		if err := config.AddAccount(config.Account{ID: id, Email: id + "@x", AccessToken: "t", Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("a1", []string{"claude-haiku-4.5", "claude-sonnet-4.5"})
	p.SetModelList("a2", []string{"claude-sonnet-4.5", "claude-opus-4.5"})
	// The pool is a process-wide singleton; drop the model lists so later tests
	// are not affected by "model known elsewhere" routing.
	t.Cleanup(func() { p.SetModelList("a1", nil); p.SetModelList("a2", nil) })

	r := newAutoRouter()
	cfg := config.DefaultAutoRouteConfig()
	cfg.Explore = 0

	d := r.Resolve(p, cfg, routeSignals{InputTokens: 100}, nil, "claude")
	if d == nil || d.Tier != "fast" || d.Model != "claude-haiku-4.5" || d.AccountID != "a1" {
		t.Fatalf("expected fast tier on a1/haiku, got %+v", d)
	}
	d = r.Resolve(p, cfg, routeSignals{Thinking: true}, nil, "claude")
	if d == nil || d.Tier != "strong" || d.Model != "claude-opus-4.5" {
		t.Fatalf("expected strong tier opus, got %+v", d)
	}

	// Balanced tier has two candidates; make a2 fail a lot and a1 should win consistently.
	for i := 0; i < 30; i++ {
		r.Record("a2", "claude-sonnet-4.5", false, 0)
		r.Record("a1", "claude-sonnet-4.5", true, 800)
	}
	wins := 0
	for i := 0; i < 20; i++ {
		if d := r.Resolve(p, cfg, routeSignals{Tools: 2}, nil, "claude"); d != nil && d.AccountID == "a1" {
			wins++
		}
	}
	if wins < 18 {
		t.Fatalf("bandit should prefer the reliable account, a1 won %d/20", wins)
	}

	// Quality slider pushes the tier up.
	cfg.QualityWeight, cfg.CostWeight = 1, 0
	if d := r.Resolve(p, cfg, routeSignals{InputTokens: 100}, nil, "claude"); d == nil || d.Tier != "balanced" {
		t.Fatalf("quality bias should lift fast→balanced, got %+v", d)
	}

	decisions, cands := r.Snapshot()
	if len(decisions) == 0 || len(cands) == 0 {
		t.Fatal("snapshot should expose decisions and candidate stats")
	}
}

func TestAutoRouterNeverFabricatesUnadvertisedModels(t *testing.T) {
	mustInitConfig(t)
	if err := config.AddAccount(config.Account{ID: "only", Email: "only@x", AccessToken: "t", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("only", []string{"claude-sonnet-4.6"})
	t.Cleanup(func() { p.SetModelList("only", nil) })

	r := newAutoRouter()
	cfg := config.DefaultAutoRouteConfig()
	cfg.Enabled = true
	cfg.Explore = 0
	cfg.Fast = []string{"claude-haiku-4.5"}
	cfg.Balanced = []string{"claude-haiku-4.5"}
	cfg.Strong = []string{"claude-haiku-4.5"}
	if d := r.Resolve(p, cfg, routeSignals{InputTokens: 100}, nil, "claude"); d != nil {
		t.Fatalf("model absent from the account catalog must not be selected: %+v", d)
	}

	// An empty model list is not permission to turn an exact tier pattern into
	// an advertised model.
	p.SetModelList("only", nil)
	if cands := r.candidates(p, []string{"claude-haiku-4.5"}, nil, config.AutoRouteConfig{}); len(cands) != 0 {
		t.Fatalf("empty account catalog produced fabricated candidates: %+v", cands)
	}
	if err := config.UpdateAutoRouteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	h := &Handler{pool: p, affinity: newAccountAffinity(), autoRouter: r}
	rec := httptest.NewRecorder()
	affinityKey := ""
	if got := h.resolveAutoModel(rec, "claude", "auto", routeSignals{InputTokens: 100}, &affinityKey, capClaudeChat); got != "auto" {
		t.Fatalf("no candidates should leave virtual model untouched, got %q", got)
	}
	if got := rec.Header().Get("X-Hekato-Routed-Model"); got != "" {
		t.Fatalf("must not announce an unvalidated concrete route, got %q", got)
	}
}

func TestMetricsCollectorQuery(t *testing.T) {
	m := newMetricsCollector()
	m.Record("claude", "claude-sonnet-4.5", "a1", true, 100, 0.5, 900)
	m.Record("claude", "claude-sonnet-4.5", "a1", true, 100, 0.5, 300)
	m.Record("openai", "claude-haiku-4.5", "a2", false, 0, 0, 0)
	points, totals, byModel, byAccount, byEndpoint := m.Query(60, 1)
	if len(points) != 60 {
		t.Fatalf("expected 60 points, got %d", len(points))
	}
	if totals.Requests != 3 || totals.Errors != 1 || totals.Tokens != 200 || totals.AvgLatency != 600 {
		t.Fatalf("unexpected totals %+v", totals)
	}
	if totals.P95 != 1000 || totals.P50 != 500 {
		t.Fatalf("unexpected percentiles p50=%d p95=%d", totals.P50, totals.P95)
	}
	if len(byModel) != 2 || byModel[0].Key != "claude-sonnet-4.5" || len(byAccount) != 2 || len(byEndpoint) != 2 {
		t.Fatalf("unexpected breakdowns %+v %+v %+v", byModel, byAccount, byEndpoint)
	}
}

func TestMetricsPersistAcrossRestart(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "")
	mustInitConfig(t)
	store := config.Metrics()
	if store == nil {
		t.Fatal("json backend should expose a metrics store")
	}
	m := newMetricsCollector()
	m.Load(store)
	m.Record("claude", "claude-sonnet-4.5", "a1", true, 120, 0.25, 700)
	m.Record("claude", "claude-sonnet-4.5", "a1", false, 0, 0, 0)
	m.Flush()

	m2 := newMetricsCollector()
	m2.Load(store)
	_, totals, byModel, _, _ := m2.Query(60, 1)
	if totals.Requests != 2 || totals.Errors != 1 || totals.Tokens != 120 || totals.AvgLatency != 700 {
		t.Fatalf("restored totals mismatch: %+v", totals)
	}
	if len(byModel) != 1 || byModel[0].Key != "claude-sonnet-4.5" || byModel[0].Requests != 2 {
		t.Fatalf("restored breakdown mismatch: %+v", byModel)
	}
}

func TestAutoRouteBlacklistIsPerProvider(t *testing.T) {
	cfg := config.AutoRouteConfig{Blacklist: []string{"codebuddy:claude-opus", " *:gemini-3 ", "haiku"}}
	if !cfg.Blacklisted(config.ProviderCodeBuddy, "claude-opus-4.8") {
		t.Fatal("codebuddy opus should be blacklisted")
	}
	if cfg.Blacklisted(config.ProviderKiro, "claude-opus-4.8") {
		t.Fatal("same model on kiro must stay eligible")
	}
	if !cfg.Blacklisted(config.ProviderGrok, "gemini-3-pro") || !cfg.Blacklisted(config.ProviderKiro, "claude-haiku-4.5") {
		t.Fatal("wildcard / bare entries should apply to every provider")
	}
}

func TestHandleModelFailureFeedsRouter(t *testing.T) {
	mustInitConfig(t)
	h := &Handler{pool: accountpool.GetPool(), autoRouter: newAutoRouter()}
	acc := &config.Account{ID: "acc-fail", Email: "f@x", Enabled: true}
	h.handleModelFailure(acc, "claude-opus-4.8", errors.New("HTTP 400 from codebuddy: model [claude-opus-4.8] service info not found"))
	_, cands := h.autoRouter.Snapshot()
	for _, c := range cands {
		if c.AccountID == "acc-fail" && c.Model == "claude-opus-4.8" && c.Failures == 1 {
			return
		}
	}
	t.Fatalf("per-attempt failure not recorded in router candidates: %+v", cands)
}

func TestPreferVisionFiltersWhenKnown(t *testing.T) {
	r := newAutoRouter()
	a := &config.Account{ID: "a"}
	cands := []routeCandidate{{account: a, model: "text-only"}, {account: a, model: "vision-model"}}
	if got := r.preferVision(cands); len(got) != 2 {
		t.Fatalf("nil vision fn must not filter, got %d", len(got))
	}
	r.vision = func(m string) bool { return m == "vision-model" }
	if got := r.preferVision(cands); len(got) != 1 || got[0].model != "vision-model" {
		t.Fatalf("expected only the vision model, got %+v", got)
	}
	r.vision = func(string) bool { return false }
	if got := r.preferVision(cands); len(got) != 2 {
		t.Fatal("unknown modality info must fall back to the full list")
	}
}
