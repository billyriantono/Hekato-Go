package providers

import "encoding/json"

// ==================== OpenAI API 类型 ====================

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
}

type OpenAIMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	// Index is only meaningful on streamed deltas, where it ties argument
	// fragments back to the call that opened them.
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolCallAccumulator merges streamed OpenAI tool_call deltas. A stream opens a
// call with {index, id, name, arguments:""} and then sends the arguments as
// fragments carrying only the index; dispatching each delta on its own emits
// the call with empty input and the fragments as nameless junk calls.
type ToolCallAccumulator struct {
	calls []ToolCall
	slot  map[int]int // delta index -> position in calls
}

func (a *ToolCallAccumulator) Add(deltas []ToolCall) {
	for _, d := range deltas {
		pos, ok := a.slot[d.Index]
		// A new id on a known index is a new call: some upstreams send complete
		// calls without an index, so every one of them arrives as index 0.
		if !ok || (d.ID != "" && a.calls[pos].ID != "" && d.ID != a.calls[pos].ID) {
			if a.slot == nil {
				a.slot = map[int]int{}
			}
			a.calls = append(a.calls, ToolCall{Type: d.Type})
			pos = len(a.calls) - 1
			a.slot[d.Index] = pos
		}
		c := &a.calls[pos]
		if d.ID != "" {
			c.ID = d.ID
		}
		if d.Function.Name != "" {
			c.Function.Name = d.Function.Name
		}
		c.Function.Arguments += d.Function.Arguments
	}
}

// Calls returns the assembled calls in the order they were opened.
func (a *ToolCallAccumulator) Calls() []ToolCall { return a.calls }

type OpenAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Parameters  interface{} `json:"parameters"`
	} `json:"function"`
}

// UnmarshalJSON accepts both the Chat Completions tool shape, where the tool
// definition is nested under "function":
//
//	{"type":"function","function":{"name":"x","description":"...","parameters":{...}}}
//
// and the Responses API tool shape, where name/description/parameters live at
// the top level:
//
//	{"type":"function","name":"x","description":"...","parameters":{...}}
//
// Without this, Responses API tools would parse with an empty Function.Name,
// which Kiro rejects with HTTP 400 "Improperly formed request".
func (t *OpenAITool) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type        string      `json:"type"`
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Parameters  interface{} `json:"parameters"`
		Function    *struct {
			Name        string      `json:"name"`
			Description string      `json:"description"`
			Parameters  interface{} `json:"parameters"`
		} `json:"function"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	t.Type = raw.Type
	if raw.Function != nil {
		t.Function.Name = raw.Function.Name
		t.Function.Description = raw.Function.Description
		t.Function.Parameters = raw.Function.Parameters
	}
	// Fall back to top-level (Responses API) fields when the nested form is
	// absent or incomplete.
	if t.Function.Name == "" {
		t.Function.Name = raw.Name
	}
	if t.Function.Description == "" {
		t.Function.Description = raw.Description
	}
	if t.Function.Parameters == nil {
		t.Function.Parameters = raw.Parameters
	}
	return nil
}

// ModelInfo is one entry of a provider model list.
type ModelInfo struct {
	ModelId        string   `json:"modelId"`
	ModelName      string   `json:"modelName"`
	Description    string   `json:"description"`
	InputTypes     []string `json:"supportedInputTypes"`
	RateMultiplier float64  `json:"rateMultiplier"`
	TokenLimits    *struct {
		MaxInputTokens  int `json:"maxInputTokens"`
		MaxOutputTokens int `json:"maxOutputTokens"`
	} `json:"tokenLimits"`
}
