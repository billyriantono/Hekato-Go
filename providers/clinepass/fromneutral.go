package clinepass

import (
	"hekato-go/providers"
)

// FromNeutral serializes a provider-neutral NeutralChat into ClinePass's
// native wire format. ClinePass speaks OpenAI Chat Completions at
// /api/v1/chat/completions, so this is the shared NeutralChat→OpenAI path
// reused by every OpenAI-shaped provider (CodeBuddy, Grok). Exposed here so
// the proxy can call clinepass.FromNeutral without reaching into the shared
// providers package directly.
func FromNeutral(nc *providers.NeutralChat) *providers.OpenAIRequest {
	return providers.NeutralToOpenAI(nc)
}