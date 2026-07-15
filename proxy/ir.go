package proxy

import "kiro-go/providers"

// ClaudeToIR parses a Claude request into the provider-neutral ChatIR. It reuses
// the shared extraction/normalization helpers (system prompt filters, thinking
// injection, content/image/tool-result extraction) so any provider consuming the
// IR gets the same normalized view without depending on Kiro's payload shape.
func ClaudeToIR(req *ClaudeRequest, thinking bool) *providers.ChatIR {
	ir := &providers.ChatIR{
		Model:       req.Model,
		System:      buildClaudeSystemPrompt(req.System, thinking),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			text, images, toolResults := extractClaudeUserContent(msg.Content)
			text = normalizeUserContent(text, len(images) > 0)
			ir.Messages = append(ir.Messages, providers.IRMessage{
				Role:        "user",
				Text:        text,
				Images:      images,
				ToolResults: toolResults,
			})
		case "assistant":
			text, toolUses := extractClaudeAssistantContent(msg.Content)
			ir.Messages = append(ir.Messages, providers.IRMessage{
				Role:      "assistant",
				Text:      text,
				ToolCalls: toolUses,
			})
		}
	}

	for _, t := range req.Tools {
		ir.Tools = append(ir.Tools, providers.IRTool{
			Name:        t.Name,
			Description: t.Description,
			Schema:      t.InputSchema,
		})
	}

	return ir
}
