package providers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponsesToolWireShape(t *testing.T) {
	var ot OpenAITool
	ot.Type = "function"
	ot.Function.Name, ot.Function.Description = "search", "web search"
	ot.Function.Parameters = map[string]interface{}{"type": "object"}
	req := ResponsesRequest{Model: "gpt-5", Tools: ToolsFromOpenAI([]OpenAITool{ot})}
	b, _ := json.Marshal(req)
	s := string(b)
	if !strings.Contains(s, `"tools":[{"type":"function","name":"search","description":"web search","parameters":{"type":"object"}}]`) {
		t.Fatalf("tools must be flat on the wire: %s", s)
	}
	// Both client shapes decode to the same thing.
	for _, in := range []string{
		`{"tools":[{"type":"function","name":"search","parameters":{}}]}`,
		`{"tools":[{"type":"function","function":{"name":"search","parameters":{}}}]}`,
	} {
		var r ResponsesRequest
		if err := json.Unmarshal([]byte(in), &r); err != nil || len(r.Tools) != 1 || r.Tools[0].Name != "search" {
			t.Fatalf("decode %s: err=%v tools=%+v", in, err, r.Tools)
		}
		back := ToolsToOpenAI(r.Tools)
		if back[0].Function.Name != "search" {
			t.Fatalf("round trip lost name: %+v", back)
		}
	}
}
