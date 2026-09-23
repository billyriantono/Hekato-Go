package proxy

import (
	"hekato-go/config"
	"hekato-go/providers"
	"hekato-go/providers/opencodego"
)

// OpenCode Go: subscription-tier ($5/mo) upstream at opencode.ai/zen/go/v1.
// Same multi-format support as OpenCode Zen (OpenAI Chat Completions,
// OpenAI Responses, Claude Messages via the shared translator). The same
// client-fingerprint quartet is required on every request.
func init() {
	registerAdapter(providerAdapter{
		kind: providerOpenCodeGo,
		chatFromClaude: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *StreamCallback) error {
			return opencodego.CallOpenAI(a, providers.NeutralToOpenAI(ClaudeToNeutral(r, thinking)), cb)
		},
		chatFromOpenAI: func(a *config.Account, r *OpenAIRequest, _ bool, cb *StreamCallback) error {
			return opencodego.CallOpenAI(a, r, cb)
		},
		responses: opencodego.CallUpstream,
		listModels: func(a *config.Account) ([]ModelInfo, error) {
			return opencodego.RefreshModels(a), nil
		},
		fetchUsage: opencodego.FetchUsage,
	})
}