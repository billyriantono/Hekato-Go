package codex

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Codex bans accounts that leak non-OpenAI agent identity strings back to
// chatgpt.com — those requests trigger the abuse pipeline that assumes the
// operator is trying to jailbreak the model into impersonating a competitor.
// Etteum-pool's scripts/proxy/sanitize.py strips the same set of markers before
// forwarding; matching that keeps our Codex tenant off the abuse list.
//
// Each pattern maps to a neutral replacement. Word boundaries (`\b`) keep the
// rewrite from over-firing on substrings (e.g. "clinician", "kira", "qed").
// We list each product exactly once — NO optional suffixes, because greedy
// tails like `claude-code`, `gemini-pro`, `grok-2-beta` consumed unrelated
// words and broke ordinary prompts in earlier revisions. Etteum-pool applies
// the same exact-match discipline in its sanitize.py.
//
// The replacements are deliberately plain: leaving nothing behind is worse
// than replacing with a generic term, because empty tool-name/user-name slots
// still surface as "the user is running <X>" in the reasoning trace.
var codexIdentityPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	// Anthropic / Claude
	{regexp.MustCompile(`(?i)\bclaude\b`), "the assistant"},
	{regexp.MustCompile(`(?i)\banthropic\b`), "the vendor"},
	// Google / Gemini
	{regexp.MustCompile(`(?i)\bgemini\b`), "the assistant"},
	// xAI / Grok
	{regexp.MustCompile(`(?i)\bgrok\b`), "the assistant"},
	{regexp.MustCompile(`(?i)\bxai\b`), "the vendor"},
	// Cursor / Cline / Roo / Continue / Zed / Windsurf — the IDE agents
	// whose system prompts brag about their own product name.
	{regexp.MustCompile(`(?i)\bcursor\b`), "the editor"},
	{regexp.MustCompile(`(?i)\bcline\b`), "the editor"},
	{regexp.MustCompile(`(?i)\broo[- ]?code\b`), "the editor"},
	{regexp.MustCompile(`(?i)\bcontinue\.dev\b`), "the editor"},
	{regexp.MustCompile(`(?i)\bwindsurf\b`), "the editor"},
	// Kiro (the one this proxy speaks natively) — never surface upstream.
	{regexp.MustCompile(`(?i)\bkiro\b`), "the editor"},
	// Amazon Q (fka CodeWhisperer)
	{regexp.MustCompile(`(?i)\bcodewhisperer\b`), "the assistant"},
	{regexp.MustCompile(`(?i)\bamazon[- ]q\b`), "the assistant"},
	// DeepSeek / Qwen / Moonshot / Kimi — Chinese vendors, same abuse risk.
	{regexp.MustCompile(`(?i)\bdeepseek\b`), "the assistant"},
	{regexp.MustCompile(`(?i)\bqwen\b`), "the assistant"},
	{regexp.MustCompile(`(?i)\bmoonshot\b`), "the vendor"},
	{regexp.MustCompile(`(?i)\bkimi\b`), "the assistant"},
}

// sanitizeIdentityText applies every codexIdentityPatterns rule to s in order.
func sanitizeIdentityText(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, p := range codexIdentityPatterns {
		out = p.re.ReplaceAllString(out, p.repl)
	}
	return out
}

// containsIdentityMarker reports whether s carries any pattern the sanitizer
// would rewrite. Cheap pre-check: we only remarshal the payload when needed.
func containsIdentityMarker(s string) bool {
	if s == "" {
		return false
	}
	// Lower-case scan against a static set — a full regex sweep here would
	// duplicate the work done by sanitizeIdentityText.
	lower := strings.ToLower(s)
	for _, needle := range identityNeedles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// identityNeedles is the pre-lowered substring set used by containsIdentityMarker.
// Kept in sync with codexIdentityPatterns above — a pattern here without a
// matching regex above (or vice versa) means the sanitizer silently misses it.
var identityNeedles = []string{
	"claude", "anthropic",
	"gemini",
	"grok", "xai",
	"cursor", "cline", "roo", "continue.dev", "windsurf",
	"kiro",
	"codewhisperer", "amazon q", "amazon-q",
	"deepseek", "qwen", "moonshot", "kimi",
}

// sanitizeCodexInput rewrites every text string inside a Codex Responses
// `input` payload. Input can be a bare string, an array of message objects,
// or an object with nested content parts — we walk it generically as JSON so
// both shapes are covered without dragging the OpenAI/Responses type hierarchy
// into this file.
//
// Returns the original bytes unchanged when no marker is present, so the
// common case pays only a single strings.Contains sweep per request.
func sanitizeCodexInput(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	if !containsIdentityMarker(string(raw)) {
		return raw
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		// Malformed input: hand back the original bytes and let the upstream
		// return its own 400 rather than silently corrupting the payload.
		return raw
	}
	cleaned := sanitizeJSONValue(v)
	out, err := json.Marshal(cleaned)
	if err != nil {
		return raw
	}
	return out
}

// sanitizeJSONValue rewrites string leaves inside a decoded JSON tree. Keys are
// left untouched (they are schema, not content) and non-string scalars pass
// through. Only string leaves in text positions matter, but sweeping every
// string is safer than trying to remember which OpenAI/Responses fields are
// user-controlled — the identity markers are unusual enough that a false
// positive on, say, a tool name is acceptable and produces "the assistant".
func sanitizeJSONValue(v interface{}) interface{} {
	switch t := v.(type) {
	case string:
		return sanitizeIdentityText(t)
	case []interface{}:
		for i, elem := range t {
			t[i] = sanitizeJSONValue(elem)
		}
		return t
	case map[string]interface{}:
		for k, elem := range t {
			t[k] = sanitizeJSONValue(elem)
		}
		return t
	default:
		return v
	}
}