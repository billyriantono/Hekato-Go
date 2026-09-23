package codex

import (
	"encoding/json"
	"hekato-go/providers"
	"strings"
	"testing"
)

// TestSanitizeIdentityText pins the rewrite table against the live Codex
// backend. Etteum-pool's scripts/proxy/sanitize.py strips the same set; we
// keep the markers in lock-step so any drift triggers a build break here before
// a real account gets flagged upstream.
func TestSanitizeIdentityText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain text", "Write a Go function", "Write a Go function"},
		{"claude", "You are Claude.", "You are the assistant."},
		{"anthropic vendor", "Powered by Anthropic", "Powered by the vendor"},
		{"gemini", "ask Gemini Pro", "ask the assistant Pro"},
		{"grok", "grok 2 beta says hi", "the assistant 2 beta says hi"},
		{"xai bare", "thanks to xAI", "thanks to the vendor"},
		{"cursor agent", "Cursor agent edit", "the editor agent edit"},
		{"cline", "cline says this", "the editor says this"},
		{"roo-code", "roo-code is here", "the editor is here"},
		{"continue.dev", "continue.dev rules", "the editor rules"},
		{"windsurf", "Windsurf user", "the editor user"},
		{"kiro", "running kiro", "running the editor"},
		{"codewhisperer", "CodeWhisperer is from AWS", "the assistant is from AWS"},
		{"amazon q", "Amazon Q: hello", "the assistant: hello"},
		{"deepseek", "DeepSeek answer", "the assistant answer"},
		{"qwen", "Qwen model", "the assistant model"},
		{"moonshot", "Moonshot AI", "the vendor AI"},
		{"kimi", "kimi reply", "the assistant reply"},
		// substring inside a longer word should NOT trigger (word-boundary kept
		// the regex strict; this is the canary for accidental over-fire).
		{"clinician stays", "the clinician", "the clinician"},
		// case-insensitive
		{"CLAUDE upper", "CLAUDE answered", "the assistant answered"},
		// multiple markers in one string
		{"combo", "I migrated from Cursor to Kiro", "I migrated from the editor to the editor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeIdentityText(tc.in)
			if got != tc.want {
				t.Fatalf("sanitizeIdentityText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestContainsIdentityMarkerPreCheck guards the fast-path. sanitizeCodexInput
// only remarshals when the pre-check fires, so this set MUST stay a subset of
// the patterns in codexIdentityPatterns.
func TestContainsIdentityMarkerPreCheck(t *testing.T) {
	for _, s := range []string{
		"running claude code",
		"thanks to Anthropic",
		"via gemini",
		"grok says hi",
		"xai has a model",
		"Cursor agent running",
		"cline edited",
		"using roo code",
		"continue.dev helped",
		"Windsurf is fast",
		"Kiro reported",
		"CodeWhisperer",
		"Amazon-Q present",
		"DeepSeek model",
		"Qwen code",
		"Moonshot AI",
		"kimi replied",
	} {
		if !containsIdentityMarker(s) {
			t.Fatalf("containsIdentityMarker(%q) = false, want true", s)
		}
	}
	for _, s := range []string{
		"",
		"plain text without any markers",
		"the clinician typed a note",
		"go is fun",
	} {
		if containsIdentityMarker(s) {
			t.Fatalf("containsIdentityMarker(%q) = true, want false", s)
		}
	}
}

// TestSanitizeCodexInputBareString covers the simplest input shape: a bare
// user-message string. The remarshal escapes the inner quotes, so the result
// must be a JSON-encoded string with the marker rewritten.
func TestSanitizeCodexInputBareString(t *testing.T) {
	raw := json.RawMessage(`"Hello Claude"`)
	got := sanitizeCodexInput(raw)
	var s string
	if err := json.Unmarshal(got, &s); err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	if s != "Hello the assistant" {
		t.Fatalf("bare string sanitized = %q, want %q", s, "Hello the assistant")
	}
}

// TestSanitizeCodexInputArrayOfMessages covers the OpenAI-Chat-Compat path used
// by CallOpenAI: input is the marshalled OpenAI messages array, so each entry
// has {role, content} where content is itself a string or an array of parts.
func TestSanitizeCodexInputArrayOfMessages(t *testing.T) {
	raw := json.RawMessage(`[
		{"role":"system","content":"You are Claude."},
		{"role":"user","content":"hi"}
	]`)
	got := sanitizeCodexInput(raw)
	var out []map[string]interface{}
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	if got := out[0]["content"].(string); !strings.Contains(got, "the assistant") {
		t.Fatalf("system prompt not sanitized: %q", got)
	}
	if got := out[1]["content"].(string); got != "hi" {
		t.Fatalf("user message altered unexpectedly: %q", got)
	}
}

// TestSanitizeCodexInputArrayOfParts covers the Responses API shape where
// `content` is an array of typed parts (`input_text`, `output_text`, …).
func TestSanitizeCodexInputArrayOfParts(t *testing.T) {
	raw := json.RawMessage(`{
		"role": "user",
		"content": [
			{"type":"input_text","text":"ask Grok"},
			{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}
		]
	}`)
	got := sanitizeCodexInput(raw)
	var out map[string]interface{}
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	parts := out["content"].([]interface{})
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	first := parts[0].(map[string]interface{})
	if got := first["text"].(string); got != "ask the assistant" {
		t.Fatalf("first part text = %q", got)
	}
	// Second part must keep its image_url subtree verbatim (we don't recurse
	// into URLs, only string leaves).
	second := parts[1].(map[string]interface{})
	if _, ok := second["image_url"]; !ok {
		t.Fatalf("image_url subtree lost: %#v", second)
	}
}

// TestSanitizeCodexInputPassthroughOnNoMarker is the fast-path: when no
// identity marker is present, the original bytes are returned untouched
// (so the upstream wire signature stays byte-for-byte identical).
func TestSanitizeCodexInputPassthroughOnNoMarker(t *testing.T) {
	raw := json.RawMessage(`{"role":"user","content":"hello world"}`)
	got := sanitizeCodexInput(raw)
	if string(got) != string(raw) {
		t.Fatalf("passthrough failed: got %s, want %s", got, raw)
	}
}

// TestSanitizeCodexInputMalformedJSONSurvivesUnchanged ensures the sanitizer
// hands malformed JSON back to the upstream (which will return its own 400)
// rather than silently zeroing it.
func TestSanitizeCodexInputMalformedJSONSurvivesUnchanged(t *testing.T) {
	// Looks like a marker but isn't valid JSON.
	raw := json.RawMessage(`{"content":"Clau`)
	got := sanitizeCodexInput(raw)
	if string(got) != string(raw) {
		t.Fatalf("malformed payload altered: %s", got)
	}
}

// TestSanitizeCodexRequestIntegration confirms the wiring at the entry point
// used by both CallUpstream and CallOpenAI: a Claude system prompt in Input
// plus a Claude-flavored Instructions string both come back sanitized, while
// unrelated fields are left alone.
func TestSanitizeCodexRequestIntegration(t *testing.T) {
	in := &providers.ResponsesRequest{
		Instructions: "you are Claude.",
		Input:        json.RawMessage(`[{"role":"user","content":"ask Gemini"}]`),
	}
	out := sanitizeCodexRequest(in)
	if !strings.Contains(out.Instructions, "the assistant") {
		t.Fatalf("Instructions not sanitized: %q", out.Instructions)
	}
	if !strings.Contains(string(out.Input), "the assistant") {
		t.Fatalf("Input not sanitized: %s", out.Input)
	}
	// store must be false (already pinned elsewhere, but here for integration).
	if out.Store == nil || *out.Store {
		t.Fatalf("store = %v, want pointer to false", out.Store)
	}
	if !out.Stream {
		t.Fatalf("stream = false, want true")
	}
}