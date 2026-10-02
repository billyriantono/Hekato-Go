package proxy

import (
	"hekato-go/config"
	"testing"
	"time"
)

func testRouteCfg() config.AutoRouteConfig {
	return config.AutoRouteConfig{
		Enabled:  true,
		Fast:     []string{"flash"},
		Balanced: []string{"glm-5.3"},
		Strong:   []string{"opus"},
	}
}

// A pin must survive an ordinary follow-up turn but break once the
// conversation classifies above the pinned model's tier — the pinned path
// carries the overwhelming majority of requests, so a stale pin is the router.
func TestPinOutranked(t *testing.T) {
	cfg := testRouteCfg()

	small := routeSignals{InputTokens: 300, Turns: 2}
	if pinOutranked(cfg, "glm-5.3", small) {
		t.Error("a small follow-up must not evict a balanced pin")
	}

	heavy := routeSignals{InputTokens: 80_000, Turns: 40, Thinking: true}
	if !pinOutranked(cfg, "glm-5.3", heavy) {
		t.Error("a strong-tier request must evict a balanced pin")
	}
	if pinOutranked(cfg, "claude-opus-4.6", heavy) {
		t.Error("a strong pin already satisfies a strong request")
	}
	// A model no tier claims was pinned deliberately; leave it alone.
	if pinOutranked(cfg, "some-operator-choice", heavy) {
		t.Error("an unclassified pin must be left alone")
	}
}

// Overlapping tier patterns must resolve to the highest matching tier, or a
// model listed in two tiers would be evicted for being in the lower one.
func TestTierOfModelTakesHighest(t *testing.T) {
	cfg := config.AutoRouteConfig{Fast: []string{"glm"}, Balanced: []string{"glm-5.3"}, Strong: []string{"opus"}}
	if got := tierOfModel(cfg, "glm-5.3"); got != 1 {
		t.Errorf("glm-5.3 matches fast and balanced; want balanced (1), got %d", got)
	}
	if got := tierOfModel(cfg, "nothing-matches"); got != -1 {
		t.Errorf("unclaimed model should report -1, got %d", got)
	}
}

// A hard rejection sidelines the pair; a plain failure does not.
func TestQuarantineAfterHardFailure(t *testing.T) {
	r := newAutoRouter()
	r.Record("acc1", "gemini-3.0-flash", false, 0)
	if r.isQuarantined("acc1", "gemini-3.0-flash") {
		t.Error("an ordinary failure must not quarantine")
	}

	r.RecordHardFailure("acc1", "gemini-3.0-flash", `{"code":11102,"msg":"model [gemini-3.0-flash] not supported"}`)
	if !r.isQuarantined("acc1", "gemini-3.0-flash") {
		t.Error("a model rejection must quarantine the pair")
	}
	if r.isQuarantined("acc2", "gemini-3.0-flash") {
		t.Error("quarantine must be per (account, model), not per model")
	}

	// It lifts on its own.
	r.mu.Lock()
	r.stats[statKey("acc1", "gemini-3.0-flash")].QuarantinedUntil = time.Now().Add(-time.Second)
	r.mu.Unlock()
	if r.isQuarantined("acc1", "gemini-3.0-flash") {
		t.Error("quarantine must expire")
	}
}

// Only messages that clearly name a model rejection may quarantine: a bare 400
// can be our own malformed request.
func TestModelRejectionClassification(t *testing.T) {
	hard := []string{
		`HTTP 400 from codebuddy: {"code":11102,"msg":"model [gemini-3.0-flash] not supported"}`,
		"commandcode HTTP 403: Model/provider not recognized: anthropic:minimax-m3",
		"upstream said: unknown model foo",
	}
	for _, m := range hard {
		if classifyError(m) != "model" {
			t.Errorf("should classify as model rejection: %s", m)
		}
	}
	soft := []string{
		`HTTP 400 from codebuddy: {"code":10001,"msg":"invalid request body"}`,
		"HTTP 429 from codebuddy: too many requests",
		"read codex sse: context deadline exceeded",
	}
	for _, m := range soft {
		if classifyError(m) == "model" {
			t.Errorf("must not quarantine on: %s", m)
		}
	}
}
