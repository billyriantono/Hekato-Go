package proxy

import (
	"hekato-go/config"
	"hekato-go/providers/modelsdev"
	"testing"
)

// A 400k-token request must not route to a 200k model, while ids the catalog
// does not know stay in the running (unknown != small).
func TestOnlyFitsContext(t *testing.T) {
	if modelsdev.ContextLimit("claude-opus-4.6") == 0 {
		t.Skip("models.dev catalog unavailable")
	}
	// The big model comes from the live catalog: a hardcoded one breaks the
	// moment any provider republishes it with a smaller window.
	big := ""
	if models, _, err := modelsdev.List(); err == nil {
		for _, m := range models {
			if modelsdev.ContextLimit(m.ID) >= 400_000+contextReserve {
				big = m.ID
				break
			}
		}
	}
	if big == "" {
		t.Skip("catalog has no model with a 400k+ window")
	}
	acc := &config.Account{ID: "a"}
	cands := []routeCandidate{
		{account: acc, model: "claude-opus-4.6"},      // 200k floor
		{account: acc, model: big},                    // fits under every spelling
		{account: acc, model: "some-private-model-x"}, // unknown
	}

	kept := onlyFitsContext(cands, 400_000)
	got := map[string]bool{}
	for _, c := range kept {
		got[c.model] = true
	}
	if got["claude-opus-4.6"] {
		t.Error("200k model kept for a 400k-token request")
	}
	if !got[big] || !got["some-private-model-x"] {
		t.Errorf("dropped a viable candidate: %v", got)
	}

	// Small request: everything fits, nothing is filtered.
	if n := len(onlyFitsContext(cands, 1000)); n != len(cands) {
		t.Errorf("small request kept %d of %d candidates", n, len(cands))
	}
	// Unknown input size disables the check entirely.
	if n := len(onlyFitsContext(cands, 0)); n != len(cands) {
		t.Errorf("zero-token request kept %d of %d candidates", n, len(cands))
	}
}

// A pinned conversation that outgrows its model's window must lose the pin.
func TestModelFitsContext(t *testing.T) {
	if modelsdev.ContextLimit("claude-opus-4.6") == 0 {
		t.Skip("models.dev catalog unavailable")
	}
	if !modelFitsContext("claude-opus-4.6", 100_000) {
		t.Error("100k request should fit a 200k model")
	}
	if modelFitsContext("claude-opus-4.6", 190_000) {
		t.Error("190k request must not fit a 200k model once the reply reserve is counted")
	}
	if !modelFitsContext("some-private-model-x", 900_000) {
		t.Error("unknown window must fail open")
	}
}
