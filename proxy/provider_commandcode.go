package proxy

import (
	"hekato-go/config"
	"hekato-go/providers"
	"hekato-go/providers/commandcode"
)

// Command Code translates its CLI AI-SDK NDJSON protocol into the shared
// StreamCallback surface. It supports Chat Completions and the assistant
// Messages route via NeutralChat; the native OpenAI Responses protocol has no
// CommandCode transport, so responses fall back through chatFromOpenAI.
func init() {
	registerAdapter(providerAdapter{
		kind: providerCommandCode,
		chatFromClaude: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *StreamCallback) error {
			return commandcode.CallOpenAI(a, providers.NeutralToOpenAI(ClaudeToNeutral(r, thinking)), cb)
		},
		chatFromOpenAI: func(a *config.Account, r *OpenAIRequest, _ bool, cb *StreamCallback) error {
			return commandcode.CallOpenAI(a, r, cb)
		},
		listModels: func(a *config.Account) ([]ModelInfo, error) {
			return commandcode.ModelsForAccount(a), nil
		},
		fetchUsage:   commandcode.FetchUsage,
		staticModels: true,
	})
}
