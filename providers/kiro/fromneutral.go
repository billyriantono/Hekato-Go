package kiro

import (
	"strings"

	"hekato-go/providers"

	"github.com/google/uuid"
)

const origin = "AI_EDITOR"

// imageOnlyUserContent stands in for a user turn that carries only images.
const imageOnlyUserContent = "Please analyze the attached image."

// FromNeutral serializes a provider-neutral chat into Kiro's wire payload.
// This is the single entry point for every client format (Claude, OpenAI,
// Responses): the proxy parses into NeutralChat, Kiro serializes from it.
//
// Shape: the system prompt becomes a priming user/assistant pair at the head of
// history; all turns but the last user turn go to history; the last user turn is
// the current message. Only one structured tool turn may stay active (the final
// history assistant's tool calls answered by the current message's tool
// results); everything else is narrated as text. Finally the payload is
// truncated to the upstream byte budget.
func FromNeutral(nc *providers.NeutralChat) *KiroPayload {
	if nc == nil {
		nc = &providers.NeutralChat{}
	}
	modelID := providers.NormalizeModelID(nc.Model)
	systemPrompt := strings.TrimSpace(nc.SystemPrompt)
	turns := mergeAdjacentUserTurns(nc.Messages)

	history := make([]KiroHistoryMessage, 0, len(turns))
	var current *providers.NeutralMessage
	if n := len(turns); n > 0 && turns[n-1].Role == "user" {
		current = &turns[n-1]
		turns = turns[:n-1]
	}
	for i := range turns {
		m := &turns[i]
		switch m.Role {
		case "user":
			u := &KiroUserInputMessage{Content: userTurnText(m), ModelID: modelID, Origin: origin}
			if len(m.Images) > 0 {
				u.Images = m.Images
			}
			if len(m.ToolResults) > 0 {
				u.UserInputMessageContext = &UserInputMessageContext{ToolResults: m.ToolResults}
			}
			history = append(history, KiroHistoryMessage{UserInputMessage: u})
		case "assistant":
			history = append(history, KiroHistoryMessage{AssistantResponseMessage: &KiroAssistantResponseMessage{
				Content: m.Text, ToolUses: m.ToolCalls,
			}})
		}
	}
	history = trimLeadingAssistantHistory(history)

	if systemPrompt != "" {
		priming := []KiroHistoryMessage{
			{UserInputMessage: &KiroUserInputMessage{Content: systemPrompt, ModelID: modelID, Origin: origin}},
			{AssistantResponseMessage: &KiroAssistantResponseMessage{Content: "I will follow these instructions."}},
		}
		history = append(priming, history...)
	}

	var currentText string
	var currentImages []Image
	var currentToolResults []ToolResult
	if current != nil {
		currentText = strings.TrimSpace(current.Text)
		currentImages = current.Images
		currentToolResults = current.ToolResults
	}

	// Only a current message that answers the final history assistant's tool
	// calls may keep structured tool results; orphans are flattened to text.
	currentToolResultIDs := collectToolResultIDs(currentToolResults)
	keepCurrentToolResults := currentToolResultsMatchLastAssistant(history, currentToolResultIDs)
	if keepCurrentToolResults {
		history = sanitizeKiroHistory(history, currentToolResultIDs)
	} else {
		history = sanitizeKiroHistory(history, nil)
	}

	finalContent := currentText
	switch {
	case len(currentToolResults) > 0 && !keepCurrentToolResults:
		finalContent = joinHistoryText(currentText, buildToolResultsContinuation(currentToolResults))
	case finalContent != "":
	case len(currentImages) > 0:
		finalContent = imageOnlyUserContent
	case len(currentToolResults) > 0:
		finalContent = buildToolResultsContinuation(currentToolResults)
	default:
		finalContent = minimalFallbackUserContent
	}

	tools, toolNameMap := ConvertTools(nc.Tools)

	payload := &KiroPayload{}
	payload.ToolNameMap = toolNameMap
	payload.ConversationState.ChatTriggerType = "MANUAL"
	payload.ConversationState.AgentTaskType = "vibe"
	payload.ConversationState.AgentContinuationId = uuid.New().String()
	payload.ConversationState.ConversationID = providers.BuildConversationID(modelID, systemPrompt, firstUserAnchor(nc.Messages))
	payload.ConversationState.CurrentMessage.UserInputMessage = KiroUserInputMessage{
		Content: finalContent, ModelID: modelID, Origin: origin, Images: currentImages,
	}
	var attach []ToolResult
	if keepCurrentToolResults {
		attach = currentToolResults
	}
	if len(tools) > 0 || len(attach) > 0 {
		payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext = &UserInputMessageContext{
			Tools: tools, ToolResults: attach,
		}
	}
	if len(history) > 0 {
		payload.ConversationState.History = history
	}
	if nc.MaxTokens > 0 || nc.Temperature > 0 || nc.TopP > 0 {
		payload.InferenceConfig = &InferenceConfig{MaxTokens: nc.MaxTokens, Temperature: nc.Temperature, TopP: nc.TopP}
	}

	truncatePayloadToLimit(payload, systemPrompt != "")
	return payload
}

// mergeAdjacentUserTurns folds consecutive tool-result-only user turns into
// one (OpenAI emits one `tool` message per result; one Kiro turn answers the
// whole tool call batch). Plain user turns are never merged: a tool-result turn
// followed by a user instruction stays two turns, and the sanitizer narrates
// the tool results into text.
func mergeAdjacentUserTurns(msgs []providers.NeutralMessage) []providers.NeutralMessage {
	isToolOnly := func(m providers.NeutralMessage) bool {
		return m.Role == "user" && strings.TrimSpace(m.Text) == "" && len(m.ToolResults) > 0
	}
	out := make([]providers.NeutralMessage, 0, len(msgs))
	for _, m := range msgs {
		if isToolOnly(m) && len(out) > 0 && isToolOnly(out[len(out)-1]) {
			last := &out[len(out)-1]
			last.Images = append(last.Images, m.Images...)
			last.ToolResults = append(last.ToolResults, m.ToolResults...)
			continue
		}
		out = append(out, m)
	}
	return out
}

func userTurnText(m *providers.NeutralMessage) string {
	text := strings.TrimSpace(m.Text)
	if text == "" && len(m.Images) > 0 {
		return imageOnlyUserContent
	}
	return text
}

// firstUserAnchor returns the first non-empty user text, used to derive a
// stable conversation ID.
func firstUserAnchor(msgs []providers.NeutralMessage) string {
	for _, m := range msgs {
		if m.Role == "user" {
			if t := strings.TrimSpace(m.Text); t != "" {
				return t
			}
		}
	}
	return ""
}

// ConvertTools turns neutral tool definitions into Kiro tool specs: names are
// sanitized to camelCase (Kiro rejects underscores) and shortened, schemas are
// cleaned. The returned map restores original names on tool_use responses.
func ConvertTools(tools []providers.NeutralTool) ([]KiroToolWrapper, map[string]string) {
	if len(tools) == 0 {
		return nil, nil
	}
	result := make([]KiroToolWrapper, 0, len(tools))
	nameMap := make(map[string]string)
	for _, tool := range tools {
		desc := tool.Description
		if len(desc) > maxToolDescLen {
			desc = desc[:maxToolDescLen] + "..."
		}
		sanitized := shortenToolName(sanitizeToolName(tool.Name))
		if sanitized != tool.Name {
			nameMap[sanitized] = tool.Name
		}
		w := KiroToolWrapper{}
		w.ToolSpecification.Name = sanitized
		w.ToolSpecification.Description = normalizeToolDesc(desc, sanitized)
		w.ToolSpecification.InputSchema = InputSchema{JSON: EnsureObjectSchema(tool.InputSchema)}
		result = append(result, w)
	}
	return result, nameMap
}

// EnsureObjectSchema is the exported form of the schema cleaner (tests).
func EnsureObjectSchema(schema interface{}) interface{} { return ensureObjectSchema(schema) }

// SanitizeHistory is the exported form of the history sanitizer (tests).
func SanitizeHistory(history []KiroHistoryMessage, currentToolResultIDs map[string]bool) []KiroHistoryMessage {
	return sanitizeKiroHistory(history, currentToolResultIDs)
}
