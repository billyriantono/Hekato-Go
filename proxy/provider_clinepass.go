package proxy

import (
	"hekato-go/config"
	"hekato-go/providers/clinepass"
)

// ClinePass is a dual-auth (API key + WorkOS OAuth) wrapper around Cline's
// OpenAI-compatible upstream. It speaks OpenAI Chat Completions only —
// /v1/chat/completions is the entry point the upstream exposes; there is no
// Responses endpoint and no Anthropic-native format. Claude callers reach it
// through the shared translator (Claude → NeutralChat → OpenAIRequest) that
// every OpenAI-shaped provider goes through.
func init() {
	registerAdapter(providerAdapter{
		kind: config.ProviderClinepass,
		// Claude requests flow NeutralChat → OpenAIRequest → upstream. We
		// reuse chatFromOpenAI for both shapes so the proxy doesn't need a
		// separate chatFromClaude entry; the proxy translator always emits
		// OpenAIRequest for OpenAI-shaped providers (see proxy/neutral.go).
		chatFromClaude: callUpstreamClinepassFromClaude,
		chatFromOpenAI: callUpstreamClinepassFromOpenAI,
		listModels:     listModelsClinepass,
		fetchUsage:     nil, // ClinePass exposes no usage endpoint yet
	})
}

// callUpstreamClinepassFromOpenAI forwards an OpenAI Chat-Completions
// request to ClinePass. The clinepass package handles the wire format.
func callUpstreamClinepassFromOpenAI(account *config.Account, req *OpenAIRequest, _ bool, cb *StreamCallback) error {
	return clinepass.CallOpenAI(account, req, cb)
}

// callUpstreamClinepassFromClaude translates a Claude request into
// NeutralChat and then into ClinePass's OpenAI-compatible wire format. The
// proxy translator handles Claude→Neutral; the clinepass package emits
// Neutral→OpenAIRequest.
func callUpstreamClinepassFromClaude(account *config.Account, req *ClaudeRequest, thinking bool, cb *StreamCallback) error {
	nc := ClaudeToNeutral(req, thinking)
	openAIReq := clinepass.FromNeutral(nc)
	return clinepass.CallOpenAI(account, openAIReq, cb)
}

func listModelsClinepass(account *config.Account) ([]ModelInfo, error) {
	return clinepass.FetchModels(account)
}