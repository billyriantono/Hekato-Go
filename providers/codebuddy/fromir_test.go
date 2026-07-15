package codebuddy

import (
	"kiro-go/providers"
	"testing"
)

func TestFromIREmitsSystemMessageNotPrimingPair(t *testing.T) {
	ir := &providers.ChatIR{
		Model:        "claude-opus-4.6",
		SystemPrompt: "You are a French tutor.",
		Messages: []providers.IRMessage{
			{Role: "user", Text: "bonjour"},
		},
	}
	req := FromIR(ir)

	if req.Model != "claude-opus-4.6" {
		t.Fatalf("model = %q", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("want system+user, got %d messages", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "You are a French tutor." {
		t.Fatalf("first message must be the real system prompt, got %+v", req.Messages[0])
	}
	if req.Messages[1].Role != "user" || req.Messages[1].Content != "bonjour" {
		t.Fatalf("second message = %+v", req.Messages[1])
	}
}

func TestFromIRToolCallAndResultRoundTrip(t *testing.T) {
	ir := &providers.ChatIR{
		Messages: []providers.IRMessage{
			{Role: "user", Text: "run ls"},
			{Role: "assistant", ToolCalls: []providers.ToolUse{
				{ToolUseID: "call_1", Name: "exec_command", Input: map[string]interface{}{"cmd": "ls"}},
			}},
			{Role: "user", ToolResults: []providers.ToolResult{
				{ToolUseID: "call_1", Content: []providers.ResultContent{{Text: "file.txt"}}},
			}},
		},
		Tools: []providers.IRTool{
			{Name: "exec_command", Description: "run", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}
	req := FromIR(ir)

	// Tool names pass through unmodified (no Kiro camelCase sanitization).
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "exec_command" {
		t.Fatalf("tool name must pass through unmodified, got %+v", req.Tools)
	}

	var sawAssistantCall, sawToolResult bool
	for _, m := range req.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) == 1 && m.ToolCalls[0].Function.Name == "exec_command" {
			sawAssistantCall = true
		}
		if m.Role == "tool" && m.ToolCallID == "call_1" && m.Content == "file.txt" {
			sawToolResult = true
		}
	}
	if !sawAssistantCall {
		t.Fatal("assistant tool call not emitted")
	}
	if !sawToolResult {
		t.Fatal("tool result not emitted as a tool message")
	}
}

func TestFromIRFallsBackToGenericSystemAndUser(t *testing.T) {
	req := FromIR(&providers.ChatIR{})
	if len(req.Messages) < 2 {
		t.Fatalf("want generic system + fallback user, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Fatalf("expected generic system fallback, got %+v", req.Messages[0])
	}
	if !hasNonSystemMessage(req.Messages) {
		t.Fatal("expected a fallback user message")
	}
	if req.Model != "auto" {
		t.Fatalf("empty IR should default model to auto, got %q", req.Model)
	}
}
