package codebuddy

import (
	"kiro-go/providers"
	"strings"
)

const fallbackUserContent = "."

// FromNeutral serializes a provider-neutral NeutralChat into CodeBuddy's native
// OpenAI-compatible /v2/chat/completions request. The generic NeutralChat→OpenAI mapping
// is shared (providers.NeutralToOpenAI); this adds CodeBuddy specifics: default model
// "auto", a generic system prompt when none is present, and a fallback user turn.
func FromNeutral(nc *providers.NeutralChat) ChatRequest {
	oai := providers.NeutralToOpenAI(nc)

	req := ChatRequest{
		Model:       oai.Model,
		Messages:    oai.Messages,
		Tools:       oai.Tools,
		MaxTokens:   oai.MaxTokens,
		Temperature: oai.Temperature,
		TopP:        oai.TopP,
		Stream:      true,
	}
	if strings.TrimSpace(req.Model) == "" {
		req.Model = "auto"
	}
	if !hasNonSystemMessage(req.Messages) {
		req.Messages = append(req.Messages, providers.OpenAIMessage{Role: "user", Content: fallbackUserContent})
	}
	req.Messages = ensureSystemMessage(req.Messages)
	return req
}

func hasNonSystemMessage(messages []providers.OpenAIMessage) bool {
	for _, m := range messages {
		if !strings.EqualFold(strings.TrimSpace(m.Role), "system") {
			return true
		}
	}
	return false
}

// ensureSystemMessage prepends a generic system prompt only when the request
// carries none (i.e. the NeutralChat had an empty SystemPrompt).
func ensureSystemMessage(messages []providers.OpenAIMessage) []providers.OpenAIMessage {
	for _, msg := range messages {
		if strings.EqualFold(strings.TrimSpace(msg.Role), "system") {
			return messages
		}
	}
	out := make([]providers.OpenAIMessage, 0, len(messages)+1)
	out = append(out, providers.OpenAIMessage{Role: "system", Content: "You are CodeBuddy, a helpful AI coding assistant."})
	return append(out, messages...)
}
