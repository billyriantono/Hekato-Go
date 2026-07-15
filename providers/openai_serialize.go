package providers

import (
	"encoding/json"
	"strings"
)

// IRToOpenAI serializes a provider-neutral ChatIR into an OpenAI Chat
// Completions request. It is the shared IR→OpenAI path reused by every provider
// that speaks OpenAI on the wire (CodeBuddy, Grok): the system prompt becomes a
// system message, tool results become `tool` messages, and tool names/schemas
// pass through unmodified.
func IRToOpenAI(ir *ChatIR) *OpenAIRequest {
	req := &OpenAIRequest{}
	if ir == nil {
		return req
	}
	req.Model = ir.Model
	req.MaxTokens = ir.MaxTokens
	req.Temperature = ir.Temperature
	req.TopP = ir.TopP

	if sys := strings.TrimSpace(ir.System); sys != "" {
		req.Messages = append(req.Messages, OpenAIMessage{Role: "system", Content: sys})
	}

	for _, m := range ir.Messages {
		switch m.Role {
		case "assistant":
			msg := OpenAIMessage{Role: "assistant", Content: m.Text}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, toolUseToOpenAI(tc))
			}
			req.Messages = append(req.Messages, msg)
		case "user":
			for _, tr := range m.ToolResults {
				req.Messages = append(req.Messages, OpenAIMessage{
					Role:       "tool",
					ToolCallID: tr.ToolUseID,
					Content:    toolResultText(tr),
				})
			}
			if strings.TrimSpace(m.Text) != "" || len(m.Images) > 0 {
				req.Messages = append(req.Messages, OpenAIMessage{
					Role:    "user",
					Content: openAIContent(m.Text, m.Images),
				})
			}
		}
	}

	for _, t := range ir.Tools {
		req.Tools = append(req.Tools, toolToOpenAI(t))
	}
	return req
}

// openAIContent returns a plain string when there are no images, or OpenAI
// multimodal content parts (text + image_url data URLs) when images are present.
func openAIContent(text string, images []Image) interface{} {
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

func toolResultText(tr ToolResult) string {
	parts := make([]string, 0, len(tr.Content))
	for _, c := range tr.Content {
		if strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func toolUseToOpenAI(tu ToolUse) ToolCall {
	args, _ := json.Marshal(tu.Input)
	tc := ToolCall{ID: tu.ToolUseID, Type: "function"}
	tc.Function.Name = tu.Name
	tc.Function.Arguments = string(args)
	return tc
}

func toolToOpenAI(t IRTool) OpenAITool {
	var ot OpenAITool
	ot.Type = "function"
	ot.Function.Name = t.Name
	ot.Function.Description = t.Description
	ot.Function.Parameters = t.Schema
	return ot
}
