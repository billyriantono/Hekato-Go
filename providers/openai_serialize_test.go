package providers

import "testing"

func TestIRToOpenAIMapsSystemToolsAndResults(t *testing.T) {
	ir := &ChatIR{
		Model:        "grok-4.5",
		SystemPrompt: "be terse",
		Messages: []IRMessage{
			{Role: "user", Text: "run ls"},
			{Role: "assistant", ToolCalls: []ToolUse{{ToolUseID: "c1", Name: "exec", Input: map[string]interface{}{"cmd": "ls"}}}},
			{Role: "user", ToolResults: []ToolResult{{ToolUseID: "c1", Content: []ResultContent{{Text: "file.txt"}}}}},
		},
		Tools: []IRTool{{Name: "exec", Description: "run", InputSchema: map[string]interface{}{"type": "object"}}},
	}
	req := IRToOpenAI(ir)

	if req.Model != "grok-4.5" {
		t.Fatalf("model = %q", req.Model)
	}
	if len(req.Messages) != 4 { // system, user, assistant(tool_call), tool
		t.Fatalf("want 4 messages, got %d: %+v", len(req.Messages), req.Messages)
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "be terse" {
		t.Fatalf("system message = %+v", req.Messages[0])
	}
	if req.Messages[2].Role != "assistant" || len(req.Messages[2].ToolCalls) != 1 || req.Messages[2].ToolCalls[0].Function.Name != "exec" {
		t.Fatalf("assistant tool call = %+v", req.Messages[2])
	}
	if req.Messages[3].Role != "tool" || req.Messages[3].ToolCallID != "c1" || req.Messages[3].Content != "file.txt" {
		t.Fatalf("tool result = %+v", req.Messages[3])
	}
	// Tool names pass through unmodified (no Kiro sanitization).
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "exec" {
		t.Fatalf("tools = %+v", req.Tools)
	}
}

func TestIRToOpenAINilSafe(t *testing.T) {
	if req := IRToOpenAI(nil); req == nil || len(req.Messages) != 0 {
		t.Fatalf("nil IR should yield empty request, got %+v", req)
	}
}
