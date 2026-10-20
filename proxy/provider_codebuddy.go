package proxy

import (
	"hekato-go/config"
	"hekato-go/providers/codebuddy"
)

// CodeBuddy (Tencent) upstream: served from NeutralChat via codebuddy.FromNeutral.
func init() {
	registerAdapter(providerAdapter{
		kind: providerCodeBuddy,
		chatFromClaude: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *StreamCallback) error {
			return codebuddy.Call(a, ClaudeToCodeBuddy(r, thinking), cb)
		},
		chatFromOpenAI: func(a *config.Account, r *OpenAIRequest, thinking bool, cb *StreamCallback) error {
			return codebuddy.Call(a, OpenAIToCodeBuddy(r, thinking), cb)
		},
		listModels: func(a *config.Account) ([]ModelInfo, error) { return codebuddy.ModelsForAccount(a), nil },
		fetchUsage: codebuddy.FetchUsage,
	})
}
