package codex

import (
	"encoding/json"
	"strings"
	"testing"

	"hekato-go/providers"
)

// ---------------------------------------------------------------------------
// resolveCodexModel
// ---------------------------------------------------------------------------

func TestResolveCodexModelKnownAlias(t *testing.T) {
	cases := []struct{ in, want string }{
		{"codex-auto", "gpt-5.3-codex"},
		{"CODEX-AUTO", "gpt-5.3-codex"},   // case-insensitive
		{"codex-gpt-5.5-xhigh", "gpt-5.5-xhigh"},
		{"codex-gpt-5.3", "gpt-5.3-codex"},
	}
	for _, tc := range cases {
		if got := resolveCodexModel(tc.in); got != tc.want {
			t.Errorf("resolveCodexModel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveCodexModelPassthrough(t *testing.T) {
	// Unknown model names pass through untouched.
	if got := resolveCodexModel("gpt-99"); got != "gpt-99" {
		t.Fatalf("expected passthrough, got %q", got)
	}
}

func TestResolveCodexModelEmpty(t *testing.T) {
	if got := resolveCodexModel(""); got != "gpt-5.3-codex" {
		t.Fatalf("empty model should default to gpt-5.3-codex, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// buildReasoning
// ---------------------------------------------------------------------------

func TestBuildReasoningDisabled(t *testing.T) {
	r := buildReasoning("anything", reasoningOptions{Disabled: true})
	if r != nil {
		t.Fatalf("disabled reasoning should return nil, got %+v", r)
	}
}

func TestBuildReasoningFromExplicitEffort(t *testing.T) {
	r := buildReasoning("gpt-5.3-codex", reasoningOptions{Effort: "high"})
	if r == nil || r.Effort != "high" {
		t.Fatalf("expected effort=high, got %+v", r)
	}
	if r.Summary != "auto" {
		t.Fatalf("expected summary=auto, got %q", r.Summary)
	}
}

func TestBuildReasoningFromBudget(t *testing.T) {
	cases := []struct {
		budget int
		effort string
	}{
		{20000, "high"},
		{8000, "medium"},
		{1000, "low"},
	}
	for _, tc := range cases {
		r := buildReasoning("gpt-5.3-codex", reasoningOptions{BudgetTokens: tc.budget})
		if r == nil || r.Effort != tc.effort {
			t.Errorf("budget=%d: expected effort=%q, got %+v", tc.budget, tc.effort, r)
		}
	}
}

func TestBuildReasoningFromModelName(t *testing.T) {
	r := buildReasoning("gpt-5.5-xhigh", reasoningOptions{})
	if r == nil || r.Effort != "xhigh" {
		t.Fatalf("expected effort=xhigh from model name, got %+v", r)
	}
}

func TestBuildReasoningNilWhenNoSignal(t *testing.T) {
	r := buildReasoning("gpt-5.3-codex", reasoningOptions{})
	if r != nil {
		t.Fatalf("expected nil when no signal, got %+v", r)
	}
}

func TestBuildReasoningWantSummary(t *testing.T) {
	r := buildReasoning("gpt-5.3-codex", reasoningOptions{Effort: "medium", WantSummary: true})
	if r == nil || r.Summary != "detailed" {
		t.Fatalf("expected summary=detailed, got %+v", r)
	}
}

// ---------------------------------------------------------------------------
// splitOpenAIMessages
// ---------------------------------------------------------------------------

func TestSplitOpenAIMessages(t *testing.T) {
	msgs := []providers.OpenAIMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "system", Content: "Additional context."},
		{Role: "user", Content: "Follow up"},
	}

	instructions, items := splitOpenAIMessages(msgs)

	if !strings.Contains(instructions, "You are helpful.") {
		t.Fatalf("instructions missing first system message: %q", instructions)
	}
	if !strings.Contains(instructions, "Additional context.") {
		t.Fatalf("instructions missing second system message: %q", instructions)
	}

	// 3 non-system messages → 3 items
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// user → message with input_text
	if items[0].Type != "message" || items[0].Role != "user" {
		t.Errorf("item 0: expected user message, got %+v", items[0])
	}
	if len(items[0].Content) != 1 || items[0].Content[0].Type != "input_text" {
		t.Errorf("item 0 content: expected input_text, got %+v", items[0].Content)
	}

	// assistant → message with output_text
	if items[1].Type != "message" || items[1].Role != "assistant" {
		t.Errorf("item 1: expected assistant message, got %+v", items[1])
	}
	if len(items[1].Content) != 1 || items[1].Content[0].Type != "output_text" {
		t.Errorf("item 1 content: expected output_text, got %+v", items[1].Content)
	}
}

func TestSplitOpenAIMessagesToolCalls(t *testing.T) {
	msgs := []providers.OpenAIMessage{
		{Role: "assistant", Content: "", ToolCalls: []providers.ToolCall{
			{
				ID:   "call_abc",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "get_weather", Arguments: `{"city":"SF"}`},
			},
		}},
		{Role: "tool", Content: `{"temp":72}`, ToolCallID: "call_abc"},
	}

	_, items := splitOpenAIMessages(msgs)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// function_call
	if items[0].Type != "function_call" {
		t.Errorf("item 0: expected function_call, got %q", items[0].Type)
	}
	if items[0].CallID != "call_abc" {
		t.Errorf("item 0: expected call_id=call_abc, got %q", items[0].CallID)
	}
	if items[0].Name != "get_weather" {
		t.Errorf("item 0: expected name=get_weather, got %q", items[0].Name)
	}

	// function_call_output
	if items[1].Type != "function_call_output" {
		t.Errorf("item 1: expected function_call_output, got %q", items[1].Type)
	}
	if items[1].CallID != "call_abc" {
		t.Errorf("item 1: expected call_id=call_abc, got %q", items[1].CallID)
	}
}

func TestSplitOpenAIMessagesAssistantTextAndToolCall(t *testing.T) {
	msgs := []providers.OpenAIMessage{
		{Role: "assistant", Content: "Let me check.", ToolCalls: []providers.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "lookup", Arguments: `{}`},
			},
		}},
	}

	_, items := splitOpenAIMessages(msgs)
	// Should produce both a text message item AND a function_call item
	if len(items) != 2 {
		t.Fatalf("expected 2 items (text + tool call), got %d", len(items))
	}
	if items[0].Type != "message" {
		t.Errorf("item 0: expected message, got %q", items[0].Type)
	}
	if items[1].Type != "function_call" {
		t.Errorf("item 1: expected function_call, got %q", items[1].Type)
	}
}

// ---------------------------------------------------------------------------
// contentToText
// ---------------------------------------------------------------------------

func TestContentToTextString(t *testing.T) {
	if got := contentToText("hello"); got != "hello" {
		t.Fatalf("expected hello, got %q", got)
	}
}

func TestContentToTextNil(t *testing.T) {
	if got := contentToText(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestContentToTextParts(t *testing.T) {
	parts := []interface{}{
		map[string]interface{}{"type": "text", "text": "part1 "},
		map[string]interface{}{"type": "input_text", "text": "part2"},
	}
	if got := contentToText(parts); got != "part1 part2" {
		t.Fatalf("expected 'part1 part2', got %q", got)
	}
}

func TestContentToTextDropsImages(t *testing.T) {
	parts := []interface{}{
		map[string]interface{}{"type": "text", "text": "visible"},
		map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "data:..."}},
	}
	if got := contentToText(parts); got != "visible" {
		t.Fatalf("expected only visible text, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// buildCodexPayload integration
// ---------------------------------------------------------------------------

func TestBuildCodexPayloadIntegration(t *testing.T) {
	req := &providers.OpenAIRequest{
		Model: "codex-auto",
		Messages: []providers.OpenAIMessage{
			{Role: "system", Content: "Be concise."},
			{Role: "user", Content: "What is Go?"},
		},
		Tools: []providers.OpenAITool{
			{Type: "function", Function: struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			}{Name: "search", Description: "web search", Parameters: map[string]interface{}{}}},
		},
	}

	out := buildCodexPayload(req, reasoningOptions{})

	// Model alias resolved
	if out.Model != "gpt-5.3-codex" {
		t.Errorf("model: got %q, want gpt-5.3-codex", out.Model)
	}

	// Instructions = system message
	if out.Instructions != "Be concise." {
		t.Errorf("instructions: got %q, want 'Be concise.'", out.Instructions)
	}

	// Input should be typed items
	var items []codexInputItem
	if err := json.Unmarshal(out.Input, &items); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 input item (user), got %d", len(items))
	}
	if items[0].Type != "message" || items[0].Role != "user" {
		t.Errorf("item: expected user message, got %+v", items[0])
	}

	// Include always present as empty array
	if out.Include == nil {
		t.Error("include should be non-nil empty slice")
	}
	if len(out.Include) != 0 {
		t.Errorf("include: got %v, want []", out.Include)
	}

	// Tool-choice auto when tools present
	if string(out.ToolChoice) != `"auto"` {
		t.Errorf("tool_choice: got %s, want \"auto\"", out.ToolChoice)
	}
	if out.ParallelToolCalls == nil || !*out.ParallelToolCalls {
		t.Error("parallel_tool_calls should be true when tools present")
	}

	// No reasoning without signal
	if out.Reasoning != nil {
		t.Errorf("reasoning: got %+v, want nil", out.Reasoning)
	}
}

func TestBuildCodexPayloadWithThinking(t *testing.T) {
	req := &providers.OpenAIRequest{
		Model: "codex-gpt-5.5-xhigh",
		Messages: []providers.OpenAIMessage{
			{Role: "user", Content: "analyze this"},
		},
	}
	out := buildCodexPayload(req, reasoningOptions{Effort: "high", WantSummary: true})

	if out.Model != "gpt-5.5-xhigh" {
		t.Errorf("model: got %q, want gpt-5.5-xhigh", out.Model)
	}
	if out.Reasoning == nil {
		t.Fatal("expected reasoning block")
	}
	if out.Reasoning.Effort != "high" {
		t.Errorf("effort: got %q, want high", out.Reasoning.Effort)
	}
	if out.Reasoning.Summary != "detailed" {
		t.Errorf("summary: got %q, want detailed", out.Reasoning.Summary)
	}
}

func TestBuildCodexPayloadNoToolsNoToolChoice(t *testing.T) {
	req := &providers.OpenAIRequest{
		Model: "gpt-5.3-codex",
		Messages: []providers.OpenAIMessage{
			{Role: "user", Content: "hi"},
		},
	}
	out := buildCodexPayload(req, reasoningOptions{})

	if out.ToolChoice != nil {
		t.Errorf("tool_choice should be nil without tools, got %s", out.ToolChoice)
	}
	if out.ParallelToolCalls != nil {
		t.Errorf("parallel_tool_calls should be nil without tools, got %v", *out.ParallelToolCalls)
	}
}

// ---------------------------------------------------------------------------
// orRandomCallID
// ---------------------------------------------------------------------------

func TestOrRandomCallIDPreservesExisting(t *testing.T) {
	if got := orRandomCallID("call_abc"); got != "call_abc" {
		t.Fatalf("expected call_abc, got %q", got)
	}
}

func TestOrRandomCallIDGeneratesForEmpty(t *testing.T) {
	got := orRandomCallID("")
	if !strings.HasPrefix(got, "call_") {
		t.Fatalf("expected call_ prefix, got %q", got)
	}
	if len(got) < 10 {
		t.Fatalf("generated id too short: %q", got)
	}
}
