package proxy

import (
	"hekato-go/config"
	"hekato-go/providers/clinepass"
	"strings"
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
		fetchUsage:     clinepass.FetchUsage,
		probeModel:     clinepassProbeModel,
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

// listModelsClinepass lists only the subscription-covered "cline-pass/*"
// models. The live /models catalog (450+ pay-per-credit entries) is
// intentionally hidden: it never contains the pass models and drowns the
// picker. Operators can still add a credit-billed model per account
// (extraModels) when they really want one.
func listModelsClinepass(account *config.Account) ([]ModelInfo, error) {
	return clinepass.ModelsForAccount(&config.Account{}), nil // static pass list
}

// clinepassProbeModel keeps Test / warmup on models the pass covers: the
// cheapest "cline-pass/*" entry (haiku / mini), else the first pass model.
func clinepassProbeModel(models []ModelInfo) string {
	var pass []ModelInfo
	for _, m := range models {
		if strings.HasPrefix(strings.ToLower(m.ModelId), "cline-pass/") {
			pass = append(pass, m)
		}
	}
	if len(pass) == 0 {
		return ""
	}
	return cheapestModel(pass)
}
