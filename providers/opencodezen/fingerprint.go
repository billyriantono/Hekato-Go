package opencodezen

import (
	"encoding/json"
	"hekato-go/providers"
	"strings"
)

// fingerprintToolNames are the two tool names the OpenCode CLI supplies to
// the Zen free-tier gate. Additional fake tools are unnecessary and alter the
// model's tool-selection surface.
var fingerprintToolNames = []string{"bash", "read"}

// InjectFingerprintTools ensures the required CLI tool pair is present in the
// tool list. Tools with matching names (case-insensitive) are renamed to the
// canonical lowercase form; missing tools are appended as minimal stubs.
// Returns the (possibly modified) tool list and a map from canonical name to
// the caller's original name, used to restore names in responses.
func InjectFingerprintTools(tools []providers.OpenAITool) ([]providers.OpenAITool, map[string]string) {
	renames := map[string]string{}
	present := map[string]bool{}

	// Pass 1: rename case-variant matches to canonical lowercase.
	for i, t := range tools {
		lower := strings.ToLower(t.Function.Name)
		for _, fp := range fingerprintToolNames {
			if lower == fp && t.Function.Name != fp {
				renames[fp] = t.Function.Name
				tools[i].Function.Name = fp
				present[fp] = true
				break
			} else if t.Function.Name == fp {
				present[fp] = true
				break
			}
		}
	}

	// Pass 2: add missing fingerprint tools as minimal stubs.
	for _, fp := range fingerprintToolNames {
		if present[fp] {
			continue
		}
		var tool providers.OpenAITool
		tool.Type = "function"
		tool.Function.Name = fp
		tool.Function.Parameters = json.RawMessage(`{"type":"object"}`)
		tools = append(tools, tool)
	}
	return tools, renames
}

// RestoreToolNames maps canonical fingerprint names back to the caller's
// original names in tool-call responses.
func RestoreToolNames(toolCalls []providers.ToolCall, renames map[string]string) []providers.ToolCall {
	if len(renames) == 0 {
		return toolCalls
	}
	for i, tc := range toolCalls {
		if orig, ok := renames[tc.Function.Name]; ok {
			toolCalls[i].Function.Name = orig
		}
	}
	return toolCalls
}

// InjectFingerprintResponsesTools is the ResponsesTool-shaped equivalent of
// InjectFingerprintTools. The two helper types differ because
// providers.ResponsesRequest uses []ResponsesTool (flat) while Chat
// Completions uses []OpenAITool (nested under "function").
func InjectFingerprintResponsesTools(tools []providers.ResponsesTool) []providers.ResponsesTool {
	present := map[string]bool{}
	for _, t := range tools {
		lower := strings.ToLower(t.Name)
		for _, fp := range fingerprintToolNames {
			if lower == fp {
				present[fp] = true
				break
			}
		}
	}
	for _, fp := range fingerprintToolNames {
		if present[fp] {
			continue
		}
		tools = append(tools, providers.ResponsesTool{
			Type:       "function",
			Name:       fp,
			Parameters: json.RawMessage(`{"type":"object"}`),
		})
	}
	return tools
}
