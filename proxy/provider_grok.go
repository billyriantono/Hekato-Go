package proxy

import (
	"hekato-go/config"
	"hekato-go/providers"
	"hekato-go/providers/grok"
)

// Grok (xAI) upstream: OpenAI-compatible chat from NeutralChat, plus a native
// Responses transport.
func init() {
	registerAdapter(providerAdapter{
		kind: providerGrok,
		chatFromClaude: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *StreamCallback) error {
			return grok.CallOpenAI(a, providers.NeutralToOpenAI(ClaudeToNeutral(r, thinking)), cb)
		},
		chatFromOpenAI: func(a *config.Account, r *OpenAIRequest, _ bool, cb *StreamCallback) error {
			return grok.CallOpenAI(a, r, cb)
		},
		responses:  grok.CallUpstream,
		listModels: func(a *config.Account) ([]ModelInfo, error) { return grok.RefreshModels(a), nil },
		fetchUsage: grok.FetchUsage,
	})
}
