package proxy

import (
	"hekato-go/config"
	accountpool "hekato-go/pool"
	"testing"
)

// Mirrors the production pool (CodeBuddy CN accounts, live model lists) and the
// operator's tier patterns; an "auto" request must resolve to a concrete model.
func TestAutoResolvesOnCodeBuddyPool(t *testing.T) {
	mustInitConfig(t)
	for _, id := range []string{"cn1", "cn2"} {
		if err := config.AddAccount(config.Account{ID: id, Email: id, AccessToken: "ck_x", RefreshToken: "ck_x", AuthMethod: "codebuddy-cn", Provider: "CodeBuddy CN", Region: "cn", Enabled: true, UsageLimit: 3200, UsageCurrent: 88}); err != nil {
			t.Fatal(err)
		}
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("cn1", []string{"glm-5.3", "glm-5.3-flash", "kimi-k2.7", "minimax-m3", "auto"})
	p.SetModelList("cn2", []string{"glm-5.2", "deepseek-v4-pro"})
	t.Cleanup(func() { p.SetModelList("cn1", nil); p.SetModelList("cn2", nil) })

	cfg := config.DefaultAutoRouteConfig()
	cfg.Enabled = true
	cfg.QualityWeight, cfg.CostWeight, cfg.SpeedWeight, cfg.Explore = 0.5, 0.55, 0.5, 0.1
	cfg.Fast = []string{"haiku", "luna", "gemini", "glm-5.3-flash"}
	cfg.Balanced = []string{"sonnet", "terra", "kimi-k3", "kimi-k2.7", "glm-5.2", "glm-5.3"}
	cfg.Strong = []string{"opus", "sol", "minimax-m3"}
	if err := config.UpdateAutoRouteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	h := &Handler{pool: p, affinity: newAccountAffinity(), autoRouter: newAutoRouter()}
	d := h.autoRouter.Resolve(p, cfg, routeSignals{InputTokens: 10}, capabilityFilter(capOpenAIChat), "openai")
	if d == nil || d.Model == "auto" {
		t.Fatalf("router found no candidate: %+v", d)
	}
	if fb := h.fallbackAutoModel(cfg, capabilityFilter(capOpenAIChat)); fb == "" || fb == "auto" {
		t.Fatalf("fallback empty: %q", fb)
	}
}
