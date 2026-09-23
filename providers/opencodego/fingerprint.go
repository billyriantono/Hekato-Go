package opencodego

import (
	"encoding/json"
	"hekato-go/providers"
	"strings"
)

// fingerprintToolNames are the four tool names required by the OpenCode
// upstream gate. Requests that omit any of them are rejected with 403.
var fingerprintToolNames = []string{"bash", "glob", "grep", "read"}

// InjectFingerprintTools ensures the fingerprint quartet is present in the
// tool list. Tools with matching names (case-insensitive) are renamed to the
// canonical lowercase form; missing tools are appended as minimal stubs.
// Returns the (possibly modified) tool list and a map from canonical name to
// the caller's original name, used to restore names in responses.
func InjectFingerprintTools(tools []providers.OpenAITool) ([]providers.OpenAITool, map[string]string) {
	renames := map[string]string{}
	present := map[string]bool{}

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

	for _, fp := range fingerprintToolNames {
		if present[fp] {
			continue
		}
		var tool providers.OpenAITool
		tool.Type = "function"
		tool.Function.Name = fp
		tool.Function.Description = fp + " tool"
		tool.Function.Parameters = json.RawMessage(`{"type":"object","properties":{}}`)
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
// InjectFingerprintTools. providers.ResponsesRequest uses []ResponsesTool
// (flat) while Chat Completions uses []OpenAITool (nested under "function").
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
			Type:        "function",
			Name:        fp,
			Description: fp + " tool",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		})
	}
	return tools
}