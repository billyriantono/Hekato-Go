package openaicompat

import "hekato-go/providers"

// FromNeutral serializes a provider-neutral NeutralChat into an OpenAI Chat
// Completions request. The wire format is identical to OpenAI, so this is the
// shared NeutralChat→OpenAI path reused by every OpenAI-shaped provider.
// Exposed here so the proxy can call openaicompat.FromNeutral without
// reaching into the shared providers package directly.
func FromNeutral(nc *providers.NeutralChat) *providers.OpenAIRequest {
	return providers.NeutralToOpenAI(nc)
}