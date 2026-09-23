package commandcode

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hekato-go/providers"
	"runtime"
	"strings"
	"time"
)

// buildParams rewrites the OpenAI Chat Completions body into the AI-SDK v5
// params envelope CommandCode expects on /alpha/generate. Differences:
//
//   - system messages collapse to params.system (a single string at top level)
//   - each message.content becomes an array of typed content blocks
//   - tool definitions use Anthropic's plain {name, description, input_schema}
//   - max_tokens/temperature fall back to safe defaults
func buildParams(req *providers.OpenAIRequest) (map[string]interface{}, error) {
	messages, system := convertMessages(req.Messages)

	params := map[string]interface{}{
		"model":       strings.TrimSpace(req.Model),
		"messages":    messages,
		"stream":      true,
		"max_tokens":  pickInt(req.MaxTokens, defaultMaxTok),
		"temperature": pickFloat(req.Temperature, defaultTemp),
	}
	if system != "" {
		params["system"] = system
	}
	if tools, err := convertTools(req.Tools); err != nil {
		return nil, err
	} else if tools != nil {
		params["tools"] = tools
	}
	if req.TopP > 0 {
		params["top_p"] = req.TopP
	}
	return params, nil
}

// convertMessages walks an OpenAI message slice and produces (messages,
// system). System messages are extracted into the system string and not
// embedded in any message turn (CommandCode rejects them inside messages[]).
func convertMessages(msgs []providers.OpenAIMessage) ([]map[string]interface{}, string) {
	out := make([]map[string]interface{}, 0, len(msgs))
	var systemParts []string

	for i := range msgs {
		m := &msgs[i]
		if m == nil {
			continue
		}
		switch m.Role {
		case "system":
			t := flattenText(m.Content)
			if t != "" {
				systemParts = append(systemParts, t)
			}
		case "tool":
			// CommandCode's tool-result blocks live on a user turn.
			out = append(out, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":       "tool-result",
						"toolCallId": m.ToolCallID,
						"toolName":   "",
						"output": map[string]interface{}{
							"type":  "text",
							"value": flattenText(m.Content),
						},
					},
				},
			})
		case "assistant":
			blocks := []map[string]interface{}{}
			// Preserve any reasoning_content (deepseek-reasoner style).
			if rc := extractReasoning(m); rc != "" {
				blocks = append(blocks, map[string]interface{}{"type": "reasoning", "text": rc})
			}
			if txt := flattenText(m.Content); txt != "" {
				blocks = append(blocks, map[string]interface{}{"type": "text", "text": txt})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, toolCallBlock(tc))
			}
			if len(blocks) == 0 {
				blocks = append(blocks, map[string]interface{}{"type": "text", "text": ""})
			}
			out = append(out, map[string]interface{}{"role": "assistant", "content": blocks})
		default:
			// user (and any other role) — normal content.
			out = append(out, map[string]interface{}{
				"role":    "user",
				"content": toContentBlocks(m.Content),
			})
		}
	}
	return out, strings.Join(systemParts, "\n\n")
}

// extractReasoning echoes back any <think>…</think> reasoning text the caller
// round-tripped via a previous assistant turn. providers.OpenAIMessage has no
// dedicated reasoning field, so the heuristic is "content looks like a think
// block"; models that never use <think> always return "".
func extractReasoning(m *providers.OpenAIMessage) string {
	if m == nil {
		return ""
	}
	text := flattenText(m.Content)
	if strings.HasPrefix(strings.TrimSpace(text), "<think>") {
		return text
	}
	return ""
}

func toolCallBlock(tc providers.ToolCall) map[string]interface{} {
	args := tc.Function.Arguments
	if args == "" {
		args = "{}"
	}
	// CommandCode accepts either a JSON string or a parsed map.
	return map[string]interface{}{
		"type":       "tool-call",
		"toolCallId": tc.ID,
		"toolName":   tc.Function.Name,
		"input":      json.RawMessage(args),
	}
}

// toContentBlocks renders OpenAI content (string | []part) as the typed block
// array CommandCode requires. Strings become a single text block; multimodal
// content emits text + image blocks preserving order.
func toContentBlocks(content interface{}) []map[string]interface{} {
	if content == nil {
		return []map[string]interface{}{{"type": "text", "text": ""}}
	}
	if s, ok := content.(string); ok {
		return []map[string]interface{}{{"type": "text", "text": s}}
	}
	arr, ok := content.([]interface{})
	if !ok {
		return []map[string]interface{}{{"type": "text", "text": flattenText(content)}}
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for _, p := range arr {
		if part, ok := p.(map[string]interface{}); ok {
			if block := partToBlock(part); block != nil {
				out = append(out, block)
				continue
			}
		}
		// Fallback: stringify unknown parts so the model still sees something.
		out = append(out, map[string]interface{}{"type": "text", "text": flattenText(p)})
	}
	if len(out) == 0 {
		out = append(out, map[string]interface{}{"type": "text", "text": ""})
	}
	return out
}

func partToBlock(part map[string]interface{}) map[string]interface{} {
	t, _ := part["type"].(string)
	switch t {
	case "text":
		if s, ok := part["text"].(string); ok {
			return map[string]interface{}{"type": "text", "text": s}
		}
	case "image_url":
		url, _ := part["image_url"].(string)
		if url == "" {
			if m, ok := part["image_url"].(map[string]interface{}); ok {
				url, _ = m["url"].(string)
			}
		}
		if url == "" {
			return nil
		}
		// CommandCode wants {type:"image", image:"data:...", mimeType:"..."}
		// even for http(s) URLs — many upstreams accept both, so pass through.
		mime := "image/png"
		if strings.HasPrefix(url, "data:") {
			if i := strings.Index(url, ";"); i > 5 {
				mime = url[5:i]
			}
		}
		return map[string]interface{}{
			"type":     "image",
			"image":    url,
			"mimeType": mime,
		}
	case "image":
		// Already in our shape.
		if s, ok := part["image"].(string); ok && s != "" {
			mime, _ := part["mimeType"].(string)
			if mime == "" {
				mime = "image/png"
			}
			return map[string]interface{}{"type": "image", "image": s, "mimeType": mime}
		}
	}
	return nil
}

// convertTools maps Chat Completions {type:function, function:{name,...}}
// entries into CommandCode's Anthropic-flavored {name, description,
// input_schema} form. Empty input_schema is replaced with {type:object} so the
// model still validates a missing schema as an object.
func convertTools(tools []providers.OpenAITool) ([]map[string]interface{}, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]map[string]interface{}, 0, len(tools))
	for i := range tools {
		t := &tools[i]
		if t.Function.Name == "" {
			continue
		}
		schema, ok := normaliseSchema(t.Function.Parameters)
		if !ok {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		out = append(out, map[string]interface{}{
			"name":         t.Function.Name,
			"description":  t.Function.Description,
			"input_schema": schema,
		})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// ---------- helpers ----------

// normaliseSchema turns the OpenAI parameters (interface{}) into a JSON
// document suitable for input_schema. nil and empty produce ("", false) so
// callers can substitute a default.
func normaliseSchema(p interface{}) (json.RawMessage, bool) {
	if p == nil {
		return nil, false
	}
	switch v := p.(type) {
	case json.RawMessage:
		if len(v) == 0 {
			return nil, false
		}
		return v, true
	case []byte:
		if len(v) == 0 {
			return nil, false
		}
		return json.RawMessage(v), true
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, false
		}
		return json.RawMessage(s), true
	}
	b, err := json.Marshal(p)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return nil, false
	}
	return b, true
}

func flattenText(content interface{}) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, p := range v {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
				continue
			}
			if m, ok := p.(map[string]interface{}); ok {
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
					continue
				}
			}
			parts = append(parts, flattenText(p))
		}
		return strings.Join(parts, "\n")
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func pickInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func pickFloat(v, def float64) float64 {
	if v <= 0 {
		return def
	}
	return v
}

// newUUID returns an RFC 4122 v4 UUID (8-4-4-4-12). Upstream validates the
// threadId strictly ("Invalid UUID"), so the layout must be exact.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		ts := uint64(time.Now().UnixNano())
		for i := range b {
			b[i] = byte(ts >> (8 * (i % 8)))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func currentDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

func runtimeOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	case "windows":
		return "win32"
	default:
		return runtime.GOOS
	}
}
