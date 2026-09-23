package proxy

import (
	"hekato-go/config"
	"hekato-go/providers/openaicompat"
)

// openaiCompatAdapter wires the OpenAI-compatible provider into the proxy.
// The OpenAI compat vendor speaks OpenAI Chat Completions at
// {BaseURL}/chat/completions, so requests are built from NeutralChat via
// openaicompat.FromNeutral (which is the shared providers.NeutralToOpenAI
// path used by every OpenAI-shaped provider). Claude-shaped clients reach
// it through the same translator (Claude → NeutralChat → OpenAIRequest) so
// the proxy doesn't need a separate chatFromClaude entry.
func init() {
	registerAdapter(providerAdapter{
		kind:           providerOpenAICompat,
		chatFromClaude: callUpstreamOpenAICompatFromClaude,
		chatFromOpenAI: callUpstreamOpenAICompatFromOpenAI,
		listModels:     func(a *config.Account) ([]ModelInfo, error) { return openaicompat.ModelsForAccount(a), nil },
		fetchUsage:     fetchUsageOpenAICompat,
	})
}

func callUpstreamOpenAICompatFromOpenAI(account *config.Account, req *OpenAIRequest, _ bool, cb *StreamCallback) error {
	return openaicompat.CallOpenAI(account, req, cb)
}

func callUpstreamOpenAICompatFromClaude(account *config.Account, req *ClaudeRequest, thinking bool, cb *StreamCallback) error {
	nc := ClaudeToNeutral(req, thinking)
	openAIReq := openaicompat.FromNeutral(nc)
	return openaicompat.CallOpenAI(account, openAIReq, cb)
}

// fetchUsageOpenAICompat returns nil usage: compatible vendors are
// pay-per-token and don't expose a usage endpoint. The proxy logs usage from
// the SSE usage block instead, so this stub satisfies the adapter contract
// without false data.
func fetchUsageOpenAICompat(_ *config.Account) (*config.AccountInfo, error) {
	return nil, nil
}
