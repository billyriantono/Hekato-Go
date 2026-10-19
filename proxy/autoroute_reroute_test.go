package proxy

import (
	"net/http/httptest"
	"testing"

	"hekato-go/config"
	accountpool "hekato-go/pool"
)

// An "auto" request whose model fails before output must move to another
// routable pair, never back to one that already failed, and stop when none is left.
func TestAutoRerouteSkipsFailedPairs(t *testing.T) {
	mustInitConfig(t)
	if err := config.AddAccount(config.Account{ID: "rr1", Email: "rr1", AccessToken: "ck_x", RefreshToken: "ck_x", AuthMethod: "codebuddy", Provider: "CodeBuddy", Region: "global", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("rr1", []string{"gpt-5.6-sol", "glm-5.3"})
	t.Cleanup(func() { p.SetModelList("rr1", nil) })

	cfg := config.DefaultAutoRouteConfig()
	cfg.Enabled = true
	cfg.Balanced = []string{"glm-5.3"}
	cfg.Strong = []string{"sol"}
	if err := config.UpdateAutoRouteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	h := &Handler{pool: p, affinity: newAccountAffinity(), autoRouter: newAutoRouter()}
	w := &logMetaWriter{ResponseWriter: httptest.NewRecorder()}
	key := ""
	h.armAutoReroute(w, "openai", routeSignals{InputTokens: 10}, &key, capOpenAIChat)

	if got := rerouteAuto(w, "rr1", "gpt-5.6-sol"); got != "glm-5.3" {
		t.Fatalf("reroute after sol failed: got %q, want glm-5.3", got)
	}
	if acc, m := h.affinity.GetWithModel(key); acc != "rr1" || m != "glm-5.3" {
		t.Fatalf("affinity not moved to the new pair: %s/%s", acc, m)
	}
	if got := rerouteAuto(w, "rr1", "glm-5.3"); got != "" {
		t.Fatalf("every pair failed, want no reroute, got %q", got)
	}
	if got := rerouteAuto(&logMetaWriter{ResponseWriter: httptest.NewRecorder()}, "rr1", "gpt-5.6-sol"); got != "" {
		t.Fatalf("explicit-model request must not reroute, got %q", got)
	}
}
