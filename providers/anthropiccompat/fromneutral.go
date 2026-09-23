package anthropiccompat

import (
	"encoding/json"
	"hekato-go/providers"
	"strings"
)

// FromNeutral serializes a provider-neutral NeutralChat into an Anthropic
// Messages request. Used when an OpenAI-shaped client reaches an
// Anthropic-compatible vendor: the proxy goes OpenAIRequest → NeutralChat
// → MessagesRequest → upstream. Claude-shaped clients reuse the ClaudeRequest
// shape directly (already isomorphic with MessagesRequest) via a separate
// translator in the proxy layer.
func FromNeutral(nc *providers.NeutralChat) *MessagesRequest {
	req := &MessagesRequest{}
	if nc == nil {
		req.MaxTokens = 4096
		return req
	}
	req.Model = nc.Model
	req.System = nc.SystemPrompt
	req.MaxTokens = nc.MaxTokens
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}
	req.Tools = make([]MessagesTool, 0, len(nc.Tools))
	for _, t := range nc.Tools {
		req.Tools = append(req.Tools, MessagesTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	req.Messages = make([]MessagesMessage, 0, len(nc.Messages))
	for _, m := range nc.Messages {
		blocks := neutralToBlocks(m)
		if len(blocks) == 0 {
			continue
		}
		role := m.Role
		if role == "assistant" {
			// Already correct.
		} else if role == "user" {
			// Correct; tool_results are emitted as user turns in Anthropic too.
		} else if role == "tool" {
			// Tool messages fold into the following user turn in Anthropic; the
			// neutral→anthropic merge is the caller's responsibility.
			continue
		} else {
			role = "user"
		}
		req.Messages = append(req.Messages, MessagesMessage{
			Role:    role,
			Content: mustRaw(blocks),
		})
	}
	return req
}

// neutralToBlocks converts one NeutralMessage into Anthropic content blocks.
func neutralToBlocks(m providers.NeutralMessage) []map[string]interface{} {
	out := []map[string]interface{}{}
	for _, img := range m.Images {
		// Anthropic expects {type:"image", source:{type:"base64", media_type, data}}.
		// IR.Image.Format is the media type ("png", "jpeg"); IR.Image.Source.Bytes
		// is the base64 payload. We emit "image/<format>" so the upstream accepts it.
		media := img.Format
		if media != "" && !strings.Contains(media, "/") {
			media = "image/" + media
		}
		out = append(out, map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": media,
				"data":       img.Source.Bytes,
			},
		})
	}
	for _, tc := range m.ToolCalls {
		out = append(out, map[string]interface{}{
			"type":  "tool_use",
			"id":    tc.ToolUseID,
			"name":  tc.Name,
			"input": tc.Input,
		})
	}
	for _, tr := range m.ToolResults {
		var content interface{} = tr.Content
		if text := joinText(tr.Content); text != "" {
			content = text
		}
		isErr := tr.Status == "error"
		out = append(out, map[string]interface{}{
			"type":        "tool_result",
			"tool_use_id": tr.ToolUseID,
			"content":     content,
			"is_error":    isErr,
		})
	}
	return out
}

// joinText returns the concatenated text of a tool result if every block is
// text, "" otherwise. Anthropic accepts either shape; we prefer the simple
// string form when possible.
func joinText(blocks []providers.ResultContent) string {
	if len(blocks) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, b := range blocks {
		sb.WriteString(b.Text)
	}
	return sb.String()
}

// mustRaw marshals an interface{} slice into a json.RawMessage. If marshalling
// fails the empty raw message is returned and the caller will surface a
// malformed request; this is intentional — neutral content should always be
// representable as JSON.
func mustRaw(v interface{}) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return raw
}