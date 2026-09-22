package proxy

import (
	"hekato-go/config"
	accountpool "hekato-go/pool"
	"testing"
)

func TestPickAccountPinsConversation(t *testing.T) {
	mustInitConfig(t)
	for _, id := range []string{"a1", "a2", "a3"} {
		if err := config.AddAccount(config.Account{ID: id, Email: id + "@x", AccessToken: "t", Enabled: true}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	p := accountpool.GetPool()
	p.Reload()
	h := &Handler{pool: p, affinity: newAccountAffinity()}

	first := h.pickAccount("conv-1", "claude-sonnet-4.5", map[string]bool{}, nil)
	if first == nil {
		t.Fatal("no account")
	}
	for i := 0; i < 5; i++ {
		if got := h.pickAccount("conv-1", "claude-sonnet-4.5", map[string]bool{}, nil); got.ID != first.ID {
			t.Fatalf("turn %d moved from %s to %s", i, first.ID, got.ID)
		}
	}

	// Pinned account failed this request → excluded → must move on and re-pin.
	next := h.pickAccount("conv-1", "claude-sonnet-4.5", map[string]bool{first.ID: true}, nil)
	if next == nil || next.ID == first.ID {
		t.Fatalf("expected a different account, got %v", next)
	}
	if got := h.pickAccount("conv-1", "claude-sonnet-4.5", map[string]bool{}, nil); got.ID != next.ID {
		t.Fatalf("expected re-pin to %s, got %s", next.ID, got.ID)
	}

	// Empty key (synthetic anchor) never pins; nil affinity is safe.
	if (&Handler{pool: p}).pickAccount("", "claude-sonnet-4.5", map[string]bool{}, nil) == nil {
		t.Fatal("unpinned pick should still return an account")
	}
}

func TestAffinityKeysStableAcrossTurns(t *testing.T) {
	turn1 := &ClaudeRequest{Model: "m", System: "sys", Messages: []ClaudeMessage{{Role: "user", Content: "first question"}}}
	turn2 := &ClaudeRequest{Model: "m", System: "sys", Messages: []ClaudeMessage{
		{Role: "user", Content: "first question"}, {Role: "assistant", Content: "answer"}, {Role: "user", Content: "follow-up"},
	}}
	if k1, k2 := claudeAffinityKey(turn1), claudeAffinityKey(turn2); k1 == "" || k1 != k2 {
		t.Fatalf("claude keys differ: %q vs %q", k1, k2)
	}
	o1 := &OpenAIRequest{Model: "m", Messages: []OpenAIMessage{{Role: "system", Content: "sys"}, {Role: "user", Content: "first question"}}}
	o2 := &OpenAIRequest{Model: "m", Messages: []OpenAIMessage{{Role: "system", Content: "sys"}, {Role: "user", Content: "first question"}, {Role: "assistant", Content: "a"}, {Role: "user", Content: "b"}}}
	if k1, k2 := openAIAffinityKey(o1), openAIAffinityKey(o2); k1 == "" || k1 != k2 {
		t.Fatalf("openai keys differ: %q vs %q", k1, k2)
	}
	if claudeAffinityKey(&ClaudeRequest{Model: "m", Messages: []ClaudeMessage{{Role: "user", Content: "."}}}) != "" {
		t.Fatal("synthetic anchor must not produce a key")
	}
}
