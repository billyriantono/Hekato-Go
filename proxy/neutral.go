package proxy

import "kiro-go/providers"

// ClaudeToNeutral parses a Claude request into the provider-neutral NeutralChat. It reuses
// the shared extraction/normalization helpers (system prompt filters, thinking
// injection, content/image/tool-result extraction) so any provider consuming the
// consumer gets the same normalized view without depending on Kiro's payload shape.
func ClaudeToNeutral(req *ClaudeRequest, thinking bool) *providers.NeutralChat {
	nc := &providers.NeutralChat{
		Model:        req.Model,
		SystemPrompt: buildClaudeSystemPrompt(req.System, thinking),
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
		TopP:         req.TopP,
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			text, images, toolResults := extractClaudeUserContent(msg.Content)
			text = normalizeUserContent(text, len(images) > 0)
			nc.Messages = append(nc.Messages, providers.NeutralMessage{
				Role:        "user",
				Text:        text,
				Images:      images,
				ToolResults: toolResults,
			})
		case "assistant":
			text, toolUses := extractClaudeAssistantContent(msg.Content)
			nc.Messages = append(nc.Messages, providers.NeutralMessage{
				Role:      "assistant",
				Text:      text,
				ToolCalls: toolUses,
			})
		}
	}

	for _, t := range req.Tools {
		nc.Tools = append(nc.Tools, providers.NeutralTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	return nc
}
