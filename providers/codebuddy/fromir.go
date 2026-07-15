package codebuddy

import (
	"encoding/json"
	"kiro-go/providers"
	"strings"
)

const fallbackUserContent = "."

// FromIR serializes a provider-neutral ChatIR into CodeBuddy's native
// OpenAI-compatible /v2/chat/completions request. Unlike the old path that
// converted from KiroPayload, this emits a clean OpenAI message list: the system
// prompt becomes a real system message (not a Kiro priming pair), tool results
// become `tool` messages, and tool names/schemas pass through unmodified (Kiro's
// camelCase sanitization does not apply to CodeBuddy).
func FromIR(ir *providers.ChatIR) ChatRequest {
	req := ChatRequest{Model: "auto", Stream: true}
	if ir == nil {
		return req
	}
	if m := strings.TrimSpace(ir.Model); m != "" {
		req.Model = m
	}
	req.MaxTokens = ir.MaxTokens
	req.Temperature = ir.Temperature
	req.TopP = ir.TopP

	if sys := strings.TrimSpace(ir.System); sys != "" {
		req.Messages = append(req.Messages, providers.OpenAIMessage{Role: "system", Content: sys})
	}

	for _, m := range ir.Messages {
		switch m.Role {
		case "assistant":
			msg := providers.OpenAIMessage{Role: "assistant", Content: m.Text}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, toolUseToOpenAI(tc))
			}
			req.Messages = append(req.Messages, msg)
		case "user":
			for _, tr := range m.ToolResults {
				req.Messages = append(req.Messages, providers.OpenAIMessage{
					Role:       "tool",
					ToolCallID: tr.ToolUseID,
					Content:    toolResultText(tr),
				})
			}
			if strings.TrimSpace(m.Text) != "" || len(m.Images) > 0 {
				req.Messages = append(req.Messages, providers.OpenAIMessage{
					Role:    "user",
					Content: content(m.Text, m.Images),
				})
			}
		}
	}

	for _, t := range ir.Tools {
		req.Tools = append(req.Tools, toolToOpenAI(t))
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
// carries none (i.e. the neutral IR had an empty System).
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

// content returns a plain string when there are no images, or OpenAI multimodal
// content parts (text + image_url data URLs) when images are present.
func content(text string, images []providers.Image) interface{} {
	if len(images) == 0 {
		return text
	}
	parts := make([]map[string]interface{}, 0, len(images)+1)
	if strings.TrimSpace(text) != "" {
		parts = append(parts, map[string]interface{}{"type": "text", "text": text})
	}
	for _, img := range images {
		format := strings.TrimSpace(img.Format)
		if format == "" {
			format = "png"
		}
		parts = append(parts, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": "data:image/" + format + ";base64," + img.Source.Bytes,
			},
		})
	}
	return parts
}

func toolResultText(tr providers.ToolResult) string {
	parts := make([]string, 0, len(tr.Content))
	for _, c := range tr.Content {
		if strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func toolUseToOpenAI(tu providers.ToolUse) providers.ToolCall {
	args, _ := json.Marshal(tu.Input)
	tc := providers.ToolCall{ID: tu.ToolUseID, Type: "function"}
	tc.Function.Name = tu.Name
	tc.Function.Arguments = string(args)
	return tc
}

func toolToOpenAI(t providers.IRTool) providers.OpenAITool {
	var ot providers.OpenAITool
	ot.Type = "function"
	ot.Function.Name = t.Name
	ot.Function.Description = t.Description
	ot.Function.Parameters = t.Schema
	return ot
}
