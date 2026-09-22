package proxy

import (
	"hekato-go/config"
	"hekato-go/providers/codex"
	"net/http"
)

// native path and the OpenAI Chat-Completions path through it. Codex has no
// Claude-format input — Claude callers see unsupportedProviderCapability and
// the dashboard's existing model override still works for custom IDs.
func init() {
	registerAdapter(providerAdapter{
		kind:      config.ProviderCodex,
		responses: callUpstreamCodexResponses,
		// chatFromOpenAI: Codex has no /v1/chat/completions endpoint, but the
		// proxy's OpenAI path accepts it and we forward via the Responses API
		// in codex.CallOpenAI. Returns nil for unsupported shapes (notably
		// multi-tool runs that don't survive the conversion); callers fall
		// back to the Responses endpoint.
		chatFromOpenAI: callUpstreamCodexFromOpenAI,
		listModels:     listModelsCodex,
		fetchUsage:     fetchUsageCodex,
	})
}

// callUpstreamCodexResponses forwards a native Responses request to Codex.
// The Codex Responses endpoint speaks the same SSE vocabulary as OpenAI's,
// so a passthrough is correct — no translation needed.
func callUpstreamCodexResponses(w http.ResponseWriter, f http.Flusher, account *config.Account, req *ResponsesRequest) error {
	return codex.CallUpstream(w, f, account, req)
}

// callUpstreamCodexFromOpenAI bridges OpenAI Chat Completions callers to
// Codex's Responses endpoint. The Codex package handles the wire-shape
// difference; this adapter is the seam.
func callUpstreamCodexFromOpenAI(account *config.Account, req *OpenAIRequest, _ bool, cb *StreamCallback) error {
	return codex.CallOpenAI(account, req, cb)
}

func listModelsCodex(account *config.Account) ([]ModelInfo, error) {
	return codex.FetchModels(account)
}

func fetchUsageCodex(account *config.Account) (*config.AccountInfo, error) {
	return codex.FetchUsage(account)
}
