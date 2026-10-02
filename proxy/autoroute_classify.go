package proxy

import (
	"hekato-go/config"
	"strings"
	"unicode/utf8"
)

// Content-derived routing signals.
//
// The counting signals alone could not tell work apart: a coding agent sends
// the same toolset on every turn, so "tools >= 8" sorted traffic into "agent"
// and "not agent" and nothing else — a 300-token "hi" and a 70k-token refactor
// landed in the same tier. These look at what the user last actually said.
//
// Only booleans derived from the text ever leave this file. The text itself is
// never stored on routeSignals, because signals are persisted in the decision
// ring and rendered in the admin panel; prompt content belongs in neither.

// lastUserText returns the final user message's text, flattened and truncated.
// Truncation is safe here: every signal below is decided by the opening of a
// message, and an unbounded copy of a 70k-token prompt has no business being
// built on the request path.
const lastUserTextLimit = 4000

var reasoningMarkers = []string{
	"step by step", "think through", "think hard", "reason about", "why does", "why is",
	"explain", "debug", "root cause", "refactor", "architect", "design a", "design the",
	"trade-off", "tradeoff", "compare", "plan the", "figure out", "investigate",
}

var simpleMarkers = []string{
	"hi", "hey", "hello", "thanks", "thank you", "ok", "okay", "yes", "no", "sure",
	"continue", "go on", "next", "done", "yep", "nope", "cool", "got it",
}

var codeMarkers = []string{"```", "func ", "class ", "def ", "import ", "SELECT ", "#include", "=>", "{}", "();"}

// contentSignals are the qualitative half of a routing decision.
type contentSignals struct {
	Code      bool
	Reasoning bool
	Simple    bool
	// KeywordTier is a tier forced by an operator keyword rule (-1 = none).
	KeywordTier int
}

// analyseText derives the content signals from the last user message.
func analyseText(text string, cfg config.AutoRouteConfig) contentSignals {
	out := contentSignals{KeywordTier: -1}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return out
	}
	lower := strings.ToLower(trimmed)

	for _, m := range codeMarkers {
		if strings.Contains(trimmed, m) {
			out.Code = true
			break
		}
	}
	for _, m := range reasoningMarkers {
		if strings.Contains(lower, m) {
			out.Reasoning = true
			break
		}
	}
	// "Simple" is about the whole message being an acknowledgement, not about
	// containing the word "ok" somewhere in a long request.
	if utf8.RuneCountInString(trimmed) <= 40 {
		stripped := strings.Trim(lower, ".!?,; \t\n")
		for _, m := range simpleMarkers {
			if stripped == m || strings.HasPrefix(stripped, m+" ") || strings.HasPrefix(stripped, m+",") {
				out.Simple = true
				break
			}
		}
	}
	out.KeywordTier = matchKeywordTier(cfg, lower)
	return out
}

// matchKeywordTier applies the operator's literal keyword rules, which win
// outright over the heuristic (LiteLLM does the same: a keyword match
// short-circuits classification). Literal only — semantic matching would mean
// an embedding model and a per-request bill to catch paraphrases that a second
// keyword catches for free.
func matchKeywordTier(cfg config.AutoRouteConfig, lowerText string) int {
	for _, rule := range cfg.KeywordRules {
		tier := tierIndexByName(rule.Tier)
		if tier < 0 {
			continue
		}
		for _, kw := range rule.Keywords {
			kw = strings.ToLower(strings.TrimSpace(kw))
			if kw != "" && strings.Contains(lowerText, kw) {
				return tier
			}
		}
	}
	return -1
}

func tierIndexByName(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "fast":
		return 0
	case "balanced":
		return 1
	case "strong":
		return 2
	}
	return -1
}

// truncateForAnalysis flattens a message to at most lastUserTextLimit bytes.
func truncateForAnalysis(s string) string {
	if len(s) <= lastUserTextLimit {
		return s
	}
	return s[:lastUserTextLimit]
}
