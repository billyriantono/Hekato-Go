package proxy

import (
	"hekato-go/config"
	"hekato-go/providers/kiro"
)

// Kiro (AWS CodeWhisperer / Amazon Q) upstream, served from NeutralChat via
// kiro.FromNeutral (ClaudeToKiro / OpenAIToKiro are thin compositions).
func init() {
	registerAdapter(providerAdapter{
		kind: providerKiro,
		chatFromClaude: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *StreamCallback) error {
			return kiro.CallAPI(a, ClaudeToKiro(r, thinking), cb)
		},
		chatFromOpenAI: func(a *config.Account, r *OpenAIRequest, thinking bool, cb *StreamCallback) error {
			return kiro.CallAPI(a, OpenAIToKiro(r, thinking), cb)
		},
		listModels: kiro.ListModels,
		fetchUsage: kiro.RefreshAccountInfo,
	})
}
