package proxy

import (
	"hekato-go/config"
	"hekato-go/pool"
	"hekato-go/providers"
	"sync"
	"time"
)

// affinityTTL matches the upstream prompt-cache lifetime: keep a conversation on
// the same account for as long as its cached prefix is likely still warm.
const affinityTTL = 5 * time.Minute

// accountAffinity pins a conversation (see conversation IDs in translator.go) to
// the account that last served it, so multi-turn sessions hit the same upstream
// prompt cache instead of being spread round-robin across accounts.
// ponytail: in-memory map, lost on restart; that only costs one cold turn.
type accountAffinity struct {
	mu      sync.Mutex
	entries map[string]affinityEntry
}

type affinityEntry struct {
	accountID string
	model     string // concrete model chosen for "auto" conversations ("" otherwise)
	expiresAt time.Time
}

func newAccountAffinity() *accountAffinity {
	return &accountAffinity{entries: make(map[string]affinityEntry)}
}

func (a *accountAffinity) Get(key string) string {
	id, _ := a.GetWithModel(key)
	return id
}

func (a *accountAffinity) GetWithModel(key string) (string, string) {
	if a == nil || key == "" {
		return "", ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		delete(a.entries, key)
		return "", ""
	}
	return e.accountID, e.model
}

// Set pins the account, preserving any model already pinned for the key.
func (a *accountAffinity) Set(key, accountID string) {
	_, model := a.GetWithModel(key)
	a.SetWithModel(key, accountID, model)
}

func (a *accountAffinity) SetWithModel(key, accountID, model string) {
	if a == nil || key == "" || accountID == "" {
		return
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.entries) > 4096 {
		for k, e := range a.entries {
			if now.After(e.expiresAt) {
				delete(a.entries, k)
			}
		}
	}
	a.entries[key] = affinityEntry{accountID: accountID, model: model, expiresAt: now.Add(affinityTTL)}
}

// pickAccount returns the pinned account for the conversation when it is still
// eligible, otherwise the next round-robin account (and pins it).
func (h *Handler) pickAccount(affinityKey, model string, excluded map[string]bool, filter pool.AccountFilter) *config.Account {
	if id := h.affinity.Get(affinityKey); id != "" && !excluded[id] {
		if acc := h.pool.GetForModelByID(id, model, filter); acc != nil {
			return acc
		}
	}
	acc := h.pool.GetNextForModelExcluding(model, excluded, filter)
	if acc != nil {
		h.affinity.Set(affinityKey, acc.ID)
	}
	return acc
}

// claudeAffinityKey derives the conversation key from the same fields as the
// upstream conversation ID. Returns "" (no pinning) for synthetic anchors.
func claudeAffinityKey(req *ClaudeRequest) string {
	anchor := firstClaudeConversationAnchor(req.Messages)
	if providers.IsSyntheticConversationAnchor(anchor) {
		return ""
	}
	return providers.BuildConversationID(req.Model, extractSystemPrompt(req.System), anchor)
}

func openAIAffinityKey(req *OpenAIRequest) string {
	var system string
	nonSystem := make([]OpenAIMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" || m.Role == "developer" {
			if s, ok := m.Content.(string); ok {
				system += s + "\n"
			}
			continue
		}
		nonSystem = append(nonSystem, m)
	}
	anchor := firstOpenAIConversationAnchor(nonSystem)
	if providers.IsSyntheticConversationAnchor(anchor) {
		return ""
	}
	return providers.BuildConversationID(req.Model, system, anchor)
}
