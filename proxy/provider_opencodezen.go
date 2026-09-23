package proxy

import (
	"hekato-go/config"
	"hekato-go/providers"
	"hekato-go/providers/opencodezen"
)

// OpenCode Zen supports both OpenAI-compatible Chat Completions and native
// Responses models. The shared handler will use the native Responses adapter
// for /v1/responses; Chat callers use the compatible transport.
func init() {
	registerAdapter(providerAdapter{
		kind: providerOpenCodeZen,
		chatFromClaude: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *StreamCallback) error {
			return opencodezen.CallOpenAI(a, providers.NeutralToOpenAI(ClaudeToNeutral(r, thinking)), cb)
		},
		chatFromOpenAI: func(a *config.Account, r *OpenAIRequest, _ bool, cb *StreamCallback) error {
			return opencodezen.CallOpenAI(a, r, cb)
		},
		responses: opencodezen.CallUpstream,
		listModels: func(a *config.Account) ([]ModelInfo, error) {
			return opencodezen.RefreshModels(a), nil
		},
		fetchUsage: opencodezen.FetchUsage,
	})
}
