package proxy

import (
	"hekato-go/config"
	"hekato-go/providers"
	"hekato-go/providers/opencodezen"
)

// OpenCode Zen: multi-format upstream at opencode.ai/zen/v1 supporting
// OpenAI Chat Completions, OpenAI Responses, and Claude Messages (the
// latter via the shared Anthropic-compatible translator).
//
// Free-tier models are gated by a client-fingerprint quartet — every
// request must declare bash/glob/grep/read tools and spoof the OpenCode
// CLI headers. The package injects both on the request side.
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