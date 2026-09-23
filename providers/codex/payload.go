package codex

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"

	"hekato-go/providers"
)

// codexModelMap resolves the client-facing model name to the slug Codex's
// backend actually accepts. Keys are lower-cased; unknown IDs pass through
// untouched so an operator can wire a new Codex slug via the dashboard's model
// override without a code change.
//
// Table mirrors etteum-pool's src/proxy/providers/codex.ts codexModelMap. Sync
// on drift — the pair `codex-auto` -> `gpt-5.3-codex` in particular masks a
// real 400 for callers hard-coded to the router-side alias.
var codexModelMap = map[string]string{
	"codex-auto":         "gpt-5.3-codex",
	"codex-gpt-5.5-xhigh": "gpt-5.5-xhigh",
	"gpt-5.5-xhigh":      "gpt-5.5-xhigh",
	"codex-gpt-5.5":      "gpt-5.5",
	"codex-gpt-5.4":      "gpt-5.4",
	"codex-gpt-5.3":      "gpt-5.3-codex",
	"codex-gpt-5.2":      "gpt-5.2",
}

// resolveCodexModel maps a request model to the Codex slug. Empty input
// returns the etteum-pool default (`gpt-5.3-codex`) so the transport layer
// never posts an empty `model` field, which Codex answers with a 400.
func resolveCodexModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "gpt-5.3-codex"
	}
	if slug, ok := codexModelMap[strings.ToLower(trimmed)]; ok {
		return slug
	}
	return trimmed
}

// codexInputItem is one entry of the Codex Responses `input` array. Codex's
// backend accepts three shapes:
//
//   - {"type":"message","role":"user|assistant|system","content":[…parts…]}
//   - {"type":"function_call","call_id","name","arguments"}
//   - {"type":"function_call_output","call_id","output"}
//
// The Chat-Completions shape (`{"role":"user","content":"…"}`) is tolerated on
// most models but confuses tool-call runs, which is exactly the failure the
// operator saw. Emitting typed items is safer and matches etteum-pool's
// buildPayload output byte-for-byte.
type codexInputItem struct {
	Type      string             `json:"type"`
	Role      string             `json:"role,omitempty"`
	Content   []codexContentPart `json:"content,omitempty"`
	CallID    string             `json:"call_id,omitempty"`
	Name      string             `json:"name,omitempty"`
	Arguments string             `json:"arguments,omitempty"`
	Output    string             `json:"output,omitempty"`
}

// codexContentPart is one content part inside a message item. `input_text` is
// used for user/system parts and `output_text` for prior assistant turns; the
// two are wire-distinct because Codex's abuse pipeline gates on the split.
type codexContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// buildCodexPayload converts a chat-completions request into the Codex
// Responses payload: system messages become `instructions`, non-system
// messages become typed `input` items, tools/tool_choice/reasoning are
// normalized. It intentionally does not run the identity sanitizer — that
// happens once in sanitizeCodexRequest so both the Responses and OpenAI paths
// share a single sanitize point.
//
// The `thinkingBudget` and `reasoningEffort` inputs are optional signals from
// the caller: budget in tokens (Claude's `thinking.budget_tokens`) or a raw
// effort string. Either may be zero/empty. `thinkingDisabled` collapses the
// whole reasoning block to nil so a client explicitly opting out gets zero
// reasoning tokens billed.
func buildCodexPayload(req *providers.OpenAIRequest, opts reasoningOptions) *providers.ResponsesRequest {
	instructions, items := splitOpenAIMessages(req.Messages)

	input, err := json.Marshal(items)
	if err != nil {
		// Marshalling a well-typed slice cannot fail in practice; on the
		// impossible error path, fall back to an empty JSON array so the
		// upstream returns a real 400 instead of the proxy panicking.
		input = json.RawMessage("[]")
	}

	out := &providers.ResponsesRequest{
		Model:        resolveCodexModel(req.Model),
		Input:        input,
		Instructions: instructions,
		Tools:        providers.ToolsFromOpenAI(req.Tools),
		Include:      []string{},
	}

	if len(req.Tools) > 0 {
		out.ToolChoice = json.RawMessage(`"auto"`)
		parallel := true
		out.ParallelToolCalls = &parallel
	}

	if r := buildReasoning(req.Model, opts); r != nil {
		out.Reasoning = r
	}

	return out
}

// reasoningOptions carries the caller-supplied hints for the reasoning block.
// Empty/zero values mean "not supplied" — buildReasoning falls through to the
// model-name inference in that case.
type reasoningOptions struct {
	// Effort is a raw effort keyword (`minimal|low|medium|high|xhigh`) taken
	// from the OpenAI `reasoning_effort` field. Normalized case-insensitively.
	Effort string
	// BudgetTokens is Claude's `thinking.budget_tokens` — larger budgets map
	// to higher effort.
	BudgetTokens int
	// Disabled forces the reasoning block off (matches etteum-pool's
	// `thinking.type === "disabled" || reasoning_effort === "none"`).
	Disabled bool
	// WantSummary requests the visible reasoning summary the CLI shows above
	// the answer (`summary: "detailed"`). False leaves summary at "auto".
	WantSummary bool
}

// buildReasoning is the etteum-pool decision tree in one place:
//
//	if disabled -> nil
//	effort = explicit || budget-derived || model-name-derived
//	summary = "detailed" when requested, else "auto"
//	nil when neither effort nor a summary preference is set
//
// The output is intentionally sparse — omitting a field lets Codex fall back
// to its own default, which is safer than pinning a value that later changes.
func buildReasoning(model string, opts reasoningOptions) *providers.ResponsesReasoning {
	if opts.Disabled {
		return nil
	}

	effort := normalizeReasoningEffort(opts.Effort)
	if effort == "" {
		effort = effortFromBudget(opts.BudgetTokens)
	}
	if effort == "" && strings.Contains(strings.ToLower(model), "xhigh") {
		effort = "xhigh"
	}

	summary := ""
	if opts.WantSummary {
		summary = "detailed"
	} else if effort != "" {
		// Etteum-pool's default when a reasoning block is emitted at all.
		summary = "auto"
	}

	if effort == "" && summary == "" {
		return nil
	}
	return &providers.ResponsesReasoning{Effort: effort, Summary: summary}
}

var validEfforts = map[string]struct{}{
	"minimal": {}, "low": {}, "medium": {}, "high": {}, "xhigh": {},
}

func normalizeReasoningEffort(raw string) string {
	e := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := validEfforts[e]; ok {
		return e
	}
	return ""
}

// effortFromBudget mirrors etteum-pool's thresholds: ≥16k -> high, ≥4k ->
// medium, else low. Zero/negative budgets return "" so the caller can fall
// through to model-name inference.
func effortFromBudget(budget int) string {
	if budget <= 0 {
		return ""
	}
	if budget >= 16000 {
		return "high"
	}
	if budget >= 4000 {
		return "medium"
	}
	return "low"
}

// splitOpenAIMessages walks the OpenAI message list once, joining every
// system message into a single instructions string (Codex accepts only one),
// and converting user/assistant/tool messages into Codex input items.
//
// The join order matches the caller's order so a later system message overrides
// an earlier one only by concatenation — this is the same policy the Claude
// path uses via extractSystemPrompt.
func splitOpenAIMessages(messages []providers.OpenAIMessage) (string, []codexInputItem) {
	var systemParts []string
	items := make([]codexInputItem, 0, len(messages))

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		text := contentToText(msg.Content)

		switch role {
		case "system", "developer":
			if strings.TrimSpace(text) != "" {
				systemParts = append(systemParts, text)
			}
		case "assistant":
			// Assistant tool calls are emitted as sibling `function_call`
			// items so Codex can resolve their outputs by call_id.
			if strings.TrimSpace(text) != "" {
				items = append(items, codexInputItem{
					Type:    "message",
					Role:    "assistant",
					Content: []codexContentPart{{Type: "output_text", Text: text}},
				})
			}
			for _, tc := range msg.ToolCalls {
				args := tc.Function.Arguments
				if args == "" {
					args = "{}"
				}
				items = append(items, codexInputItem{
					Type:      "function_call",
					CallID:    orRandomCallID(tc.ID),
					Name:      tc.Function.Name,
					Arguments: args,
				})
			}
		case "tool":
			items = append(items, codexInputItem{
				Type:   "function_call_output",
				CallID: msg.ToolCallID,
				Output: text,
			})
		default: // "user" and unknown
			items = append(items, codexInputItem{
				Type:    "message",
				Role:    "user",
				Content: []codexContentPart{{Type: "input_text", Text: text}},
			})
		}
	}

	return strings.Join(systemParts, "\n"), items
}

// contentToText flattens an OpenAI message `content` field into a plain
// string, tolerating the three shapes callers send in the wild:
//
//   - bare string: returned as-is
//   - array of parts: each part's `.text` field concatenated
//   - object: JSON-stringified as a last resort (Codex won't accept it, but
//     the resulting 400 is more diagnosable than silently sending "")
//
// Image parts (`image_url` / `input_image`) are dropped: Codex's Responses
// endpoint on chatgpt.com does not accept image content on the codex/responses
// route, and etteum-pool drops them the same way.
func contentToText(content interface{}) string {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	if parts, ok := content.([]interface{}); ok {
		var sb strings.Builder
		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if !ok {
				if s, ok := p.(string); ok {
					sb.WriteString(s)
				}
				continue
			}
			pType, _ := part["type"].(string)
			switch pType {
			case "text", "input_text", "output_text":
				if t, ok := part["text"].(string); ok {
					sb.WriteString(t)
				}
			case "tool_result":
				if t, ok := part["content"].(string); ok {
					sb.WriteString(t)
				}
			default:
				// Unknown part types (image_url, refusal, …) fall through.
				if t, ok := part["text"].(string); ok {
					sb.WriteString(t)
				}
			}
		}
		return sb.String()
	}
	if m, ok := content.(map[string]interface{}); ok {
		if t, ok := m["text"].(string); ok {
			return t
		}
		if raw, err := json.Marshal(m); err == nil {
			return string(raw)
		}
	}
	return ""
}

// orRandomCallID keeps Codex's function_call/function_call_output pairing
// working when the upstream client omitted the id. Codex rejects duplicate
// call_ids, so a random one is the only safe fallback.
func orRandomCallID(id string) string {
	if strings.TrimSpace(id) != "" {
		return id
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "call_synthetic"
	}
	return "call_" + hex.EncodeToString(buf[:])
}
