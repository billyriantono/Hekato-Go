package providers

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// MinimalFallbackUserContent is the placeholder sent when a user turn has no
// usable text (providers reject empty content).
const MinimalFallbackUserContent = "."

// modelAliases lists model names that need an explicit redirect — dated snapshots,
// cross-family legacy IDs (claude-3-*), and non-Anthropic fallbacks.
// Plain dash → dot version normalization is handled by claudeVersionPattern below,
// so new versions (e.g. claude-opus-4-8) require no code changes.
type modelMapping struct {
	key   string
	value string
}

var modelAliases = []modelMapping{
	{"claude-sonnet-4-20250514", "claude-sonnet-4"},
	{"claude-3-5-sonnet", "claude-sonnet-4.5"},
	{"claude-3-opus", "claude-sonnet-4.5"},
	{"claude-3-sonnet", "claude-sonnet-4"},
	{"claude-3-haiku", "claude-haiku-4.5"},
	{"gpt-4-turbo", "claude-sonnet-4.5"},
	{"gpt-4o", "claude-sonnet-4.5"},
	{"gpt-4", "claude-sonnet-4.5"},
	{"gpt-3.5-turbo", "claude-sonnet-4.5"},
}

// claudeVersionPattern normalizes "claude-{family}-N-M" to "claude-{family}-N.M".
// Minor is capped at 1-2 digits with a \b boundary so dated snapshots
// (claude-sonnet-4-20250514) are not accidentally rewritten.
var claudeVersionPattern = regexp.MustCompile(`claude-(opus|sonnet|haiku)-(\d+)-(\d{1,2})\b`)

// NormalizeModelID maps client-facing model IDs (aliases, dated snapshots,
// "claude-sonnet-4-5" dash form) onto the canonical IDs upstreams accept.
// Unknown IDs pass through unchanged so new models need no code change.
func NormalizeModelID(model string) string {
	lower := strings.ToLower(model)
	for _, m := range modelAliases {
		if strings.Contains(lower, m.key) {
			return m.value
		}
	}
	if claudeVersionPattern.MatchString(lower) {
		return claudeVersionPattern.ReplaceAllString(lower, "claude-$1-$2.$3")
	}
	return model
}

// BuildConversationID derives a stable conversation ID from the model, the
// system prompt and the first real user message, so every turn of a session
// hashes to the same upstream conversation (prompt-cache friendly). Synthetic
// anchors (placeholders) get a random ID instead.
func BuildConversationID(modelID, systemPrompt, anchor string) string {
	anchor = strings.TrimSpace(anchor)
	if IsSyntheticConversationAnchor(anchor) {
		return uuid.New().String()
	}
	seed := strings.Join([]string{modelID, strings.TrimSpace(systemPrompt), anchor}, "\n")
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(seed)).String()
}

// IsSyntheticConversationAnchor reports whether a first-user-message text is a
// placeholder rather than real user input.
func IsSyntheticConversationAnchor(anchor string) bool {
	if strings.TrimSpace(anchor) == "" {
		return true
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(anchor), " "))
	switch normalized {
	case ".", "begin conversation", "please analyze the attached image.", strings.ToLower(MinimalFallbackUserContent):
		return true
	default:
		return false
	}
}
