package codebuddy

import (
	"hekato-go/config"
	"hekato-go/providers"
	"net/http"
	"testing"
)

func TestFromIREmitsSystemMessageNotPrimingPair(t *testing.T) {
	ir := &providers.NeutralChat{
		Model:        "claude-opus-4.6",
		SystemPrompt: "You are a French tutor.",
		Messages: []providers.NeutralMessage{
			{Role: "user", Text: "bonjour"},
		},
	}
	req := FromNeutral(ir)

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
	ir := &providers.NeutralChat{
		Messages: []providers.NeutralMessage{
			{Role: "user", Text: "run ls"},
			{Role: "assistant", ToolCalls: []providers.ToolUse{
				{ToolUseID: "call_1", Name: "exec_command", Input: map[string]interface{}{"cmd": "ls"}},
			}},
			{Role: "user", ToolResults: []providers.ToolResult{
				{ToolUseID: "call_1", Content: []providers.ResultContent{{Text: "file.txt"}}},
			}},
		},
		Tools: []providers.NeutralTool{
			{Name: "exec_command", Description: "run", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}
	req := FromNeutral(ir)

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
	req := FromNeutral(&providers.NeutralChat{})
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
		t.Fatalf("empty input should default model to auto, got %q", req.Model)
	}
}

func TestApiKeyHeaderOnlyForRealKeys(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.sig"
	h := http.Header{}
	applyCodeBuddyHeaders(h, codeBuddyGlobal, jwt)
	if h.Get("X-API-Key") != "" {
		t.Fatal("JWT session tokens must not be sent as X-API-Key (CodeBuddy answers 401 not_found)")
	}
	if h.Get("Authorization") != "Bearer "+jwt {
		t.Fatalf("unexpected Authorization %q", h.Get("Authorization"))
	}
	h = http.Header{}
	applyCodeBuddyHeaders(h, codeBuddyGlobal, "sk-real-key")
	if h.Get("X-API-Key") != "sk-real-key" {
		t.Fatal("real API keys should still be sent as X-API-Key")
	}
}

func TestModelsForAccountUsesPublishedStaticCatalogs(t *testing.T) {
	global := ModelsForAccount(&config.Account{ProviderKind: "codebuddy"})
	cn := ModelsForAccount(&config.Account{ProviderKind: "codebuddy", Region: "cn"})
	has := func(models []providers.ModelInfo, want string) bool {
		for _, model := range models {
			if model.ModelId == want {
				return true
			}
		}
		return false
	}
	if !has(global, "claude-opus-4.8") || !has(global, "gpt-5.5-xhigh") || !has(global, "gemini-3.5-flash") {
		t.Fatalf("global catalog missing representative models: %+v", global)
	}
	if has(global, "claude-haiku-4.5") {
		t.Fatal("global catalog must not publish unavailable claude-haiku-4.5")
	}
	for _, model := range []string{"glm-5.3", "glm-5.3-flash", "kimi-k3-1", "hy3", "hy4-preview", "deepseek-v4.1-flash"} {
		if !has(cn, model) {
			t.Fatalf("CN catalog missing %q: %+v", model, cn)
		}
	}
	if has(cn, "deepseek-v4-flash") {
		t.Fatal("CN catalog must not publish replaced deepseek-v4-flash")
	}
}

// TestIsCodeBuddyCNToken pins region detection from the token's issuer. A CN
// credential sent to the global gateway answers 401 from the APISIX edge, so
// misclassifying one silently yields a dead account. Detection must key off the
// issuer realm, never the token's shape (both regions issue Keycloak JWTs).
func TestIsCodeBuddyCNToken(t *testing.T) {
	// Real CN issuer: www.codebuddy.cn realm. Header/payload are valid base64url.
	cnJWT := "eyJhbGciOiJSUzI1NiJ9." +
		"eyJpc3MiOiJodHRwczovL3d3dy5jb2RlYnVkZHkuY24vYXV0aC9yZWFsbXMvY29waWxvdCIsInN1YiI6ImFiYyJ9" +
		".sig"
	if !isCodeBuddyCNToken(cnJWT) {
		t.Fatal("codebuddy.cn issuer must be detected as CN")
	}

	// Older CN exports were issued by the copilot.tencent.com realm.
	tencentJWT := "eyJhbGciOiJSUzI1NiJ9." +
		"eyJpc3MiOiJodHRwczovL2NvcGlsb3QudGVuY2VudC5jb20vYXV0aC9yZWFsbXMvY29waWxvdCJ9" +
		".sig"
	if !isCodeBuddyCNToken(tencentJWT) {
		t.Fatal("copilot.tencent.com issuer must be detected as CN")
	}

	// A global issuer must not be misclassified.
	globalJWT := "eyJhbGciOiJSUzI1NiJ9." +
		"eyJpc3MiOiJodHRwczovL3d3dy5jb2RlYnVkZHkuYWkvYXV0aC9yZWFsbXMvY29waWxvdCJ9" +
		".sig"
	if isCodeBuddyCNToken(globalJWT) {
		t.Fatal("codebuddy.ai issuer must not be detected as CN")
	}

	// Plain API keys and malformed tokens carry no issuer — the operator's
	// explicit variant decides.
	for _, tok := range []string{"ck_live_abc123", "", "not-a-jwt", "eyJhbGciOiJIUzI1NiJ9"} {
		if isCodeBuddyCNToken(tok) {
			t.Fatalf("non-JWT credential %q must not be detected as CN", tok)
		}
	}
}
