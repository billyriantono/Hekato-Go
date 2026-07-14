// Request-format converters for the CodeBuddy provider. They live in the
// proxy core because they reuse the shared Claude/OpenAI normalization layer
// (ClaudeToKiro and friends); the transport itself is providers/codebuddy.
package proxy

import (
	"encoding/json"
	"kiro-go/config"
	"kiro-go/providers/codebuddy"
	"strings"
)

func isCodeBuddyAccount(account *config.Account) bool {
	return mustBeProvider(account, providerCodeBuddy)
}

// ClaudeToCodeBuddy converts a Claude-format request to CodeBuddy's native
// OpenAI-compatible /v2/chat/completions body. It intentionally reuses the
// existing ClaudeToKiro normalizer so prompt/tool/image behavior stays identical
// across Kiro and CodeBuddy until we split the shared normalization layer.
func ClaudeToCodeBuddy(req *ClaudeRequest, thinking bool) codebuddy.ChatRequest {
	out := codeBuddyFromNormalizedPayload(ClaudeToKiro(req, thinking))
	// CodeBuddy's CLI path is streaming-first. We always request upstream SSE and
	// aggregate locally for non-stream downstream clients.
	out.Stream = true
	out.StreamOptions = map[string]bool{"include_usage": true}
	return out
}

// OpenAIToCodeBuddy converts an OpenAI Chat Completions request to CodeBuddy's
// native /v2/chat/completions body. Match enowx's CodeBuddy provider: OpenAI
// chat requests stay OpenAI-on-the-wire instead of passing through Kiro's
// payload shape first.
func OpenAIToCodeBuddy(req *OpenAIRequest, thinking bool) codebuddy.ChatRequest {
	if req == nil {
		return codebuddy.ChatRequest{Model: "auto", Stream: false}
	}
	out := codebuddy.ChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		// CodeBuddy Global accepts the same streaming shape used by the CLI/enowx
		// common path. Non-stream client responses are aggregated by our handler.
		Stream:        true,
		StreamOptions: map[string]bool{"include_usage": true},
		Tools:         req.Tools,
	}
	if len(out.Messages) == 0 {
		out.Messages = []OpenAIMessage{{Role: "user", Content: minimalFallbackUserContent}}
	}
	out.Messages = ensureCodeBuddySystemMessage(out.Messages)
	return out
}

func codeBuddyFromNormalizedPayload(payload *KiroPayload) codebuddy.ChatRequest {
	model := "auto"
	if payload != nil {
		if m := strings.TrimSpace(payload.ConversationState.CurrentMessage.UserInputMessage.ModelID); m != "" {
			model = m
		}
	}

	req := codebuddy.ChatRequest{
		Model:  model,
		Stream: true,
	}
	if payload == nil {
		return req
	}
	if payload.InferenceConfig != nil {
		req.MaxTokens = payload.InferenceConfig.MaxTokens
		req.Temperature = payload.InferenceConfig.Temperature
		req.TopP = payload.InferenceConfig.TopP
	}

	for _, h := range payload.ConversationState.History {
		appendKiroHistoryAsOpenAI(&req.Messages, h)
	}
	current := payload.ConversationState.CurrentMessage.UserInputMessage
	appendKiroUserAsOpenAI(&req.Messages, current)
	if current.UserInputMessageContext != nil {
		req.Tools = convertKiroToolsToOpenAI(current.UserInputMessageContext.Tools)
	}
	if len(req.Messages) == 0 {
		req.Messages = append(req.Messages, OpenAIMessage{Role: "user", Content: minimalFallbackUserContent})
	}
	req.Messages = ensureCodeBuddySystemMessage(req.Messages)
	return req
}

func ensureCodeBuddySystemMessage(messages []OpenAIMessage) []OpenAIMessage {
	for _, msg := range messages {
		if strings.EqualFold(strings.TrimSpace(msg.Role), "system") {
			return messages
		}
	}
	out := make([]OpenAIMessage, 0, len(messages)+1)
	out = append(out, OpenAIMessage{Role: "system", Content: "You are CodeBuddy, a helpful AI coding assistant."})
	out = append(out, messages...)
	return out
}

func appendKiroHistoryAsOpenAI(messages *[]OpenAIMessage, h KiroHistoryMessage) {
	if h.UserInputMessage != nil {
		appendKiroUserAsOpenAI(messages, *h.UserInputMessage)
	}
	if h.AssistantResponseMessage != nil {
		msg := OpenAIMessage{Role: "assistant", Content: h.AssistantResponseMessage.Content}
		for _, tu := range h.AssistantResponseMessage.ToolUses {
			msg.ToolCalls = append(msg.ToolCalls, kiroToolUseToOpenAI(tu))
		}
		*messages = append(*messages, msg)
	}
}

func appendKiroUserAsOpenAI(messages *[]OpenAIMessage, u KiroUserInputMessage) {
	if u.UserInputMessageContext != nil {
		for _, tr := range u.UserInputMessageContext.ToolResults {
			*messages = append(*messages, OpenAIMessage{
				Role:       "tool",
				ToolCallID: tr.ToolUseID,
				Content:    kiroToolResultText(tr),
			})
		}
	}
	content := strings.TrimSpace(u.Content)
	if content == "" || strings.HasPrefix(content, toolResultsContinuationPrefix) {
		return
	}
	*messages = append(*messages, OpenAIMessage{Role: "user", Content: codeBuddyContent(u.Content, u.Images)})
}

func codeBuddyContent(text string, images []KiroImage) interface{} {
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

func kiroToolResultText(tr KiroToolResult) string {
	parts := make([]string, 0, len(tr.Content))
	for _, c := range tr.Content {
		if strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func kiroToolUseToOpenAI(tu KiroToolUse) ToolCall {
	args, _ := json.Marshal(tu.Input)
	tc := ToolCall{ID: tu.ToolUseID, Type: "function"}
	tc.Function.Name = tu.Name
	tc.Function.Arguments = string(args)
	return tc
}

func convertKiroToolsToOpenAI(tools []KiroToolWrapper) []OpenAITool {
	out := make([]OpenAITool, 0, len(tools))
	for _, kt := range tools {
		var ot OpenAITool
		ot.Type = "function"
		ot.Function.Name = kt.ToolSpecification.Name
		ot.Function.Description = kt.ToolSpecification.Description
		ot.Function.Parameters = kt.ToolSpecification.InputSchema.JSON
		out = append(out, ot)
	}
	return out
}
