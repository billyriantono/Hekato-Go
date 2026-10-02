package providers

import "encoding/json"

type ResponsesRequest struct {
	Model              string            `json:"model"`
	Input              json.RawMessage   `json:"input"`
	Instructions       string            `json:"instructions,omitempty"`
	Stream             bool              `json:"stream,omitempty"`
	Tools              []ResponsesTool   `json:"tools,omitempty"`
	ToolChoice         json.RawMessage   `json:"tool_choice,omitempty"`
	PreviousResponseID string            `json:"previous_response_id,omitempty"`
	Store              *bool             `json:"store,omitempty"`
	Temperature        *float64          `json:"temperature,omitempty"`
	MaxOutputTokens    *int              `json:"max_output_tokens,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	// Reasoning is the Codex/Responses `{effort, summary}` block. Optional on
	// the public Responses API; Codex accepts it and adjusts its reasoning
	// budget accordingly. Serialized as-is so a native Responses caller who
	// already supplied it round-trips unchanged.
	Reasoning *ResponsesReasoning `json:"reasoning,omitempty"`
	// Include is Codex's explicit include-list. The upstream is tolerant of
	// omission but etteum-pool always sends `include:[]` — matching that
	// avoids the case where a future model revision starts treating "absent"
	// and "empty" differently.
	Include []string `json:"include,omitempty"`
	// ParallelToolCalls surfaces the caller's tool-parallelism preference to
	// Codex. A nil pointer omits the field; a false pointer forces sequential
	// tool use even when tools are present.
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`
}

// ResponsesTool is a tool in Responses-API wire shape: flat, with name /
// description / parameters at the top level (Chat Completions nests them
// under "function"). Codex rejects the nested form with
// "Missing required parameter: 'tools[0].name'".
type ResponsesTool struct {
	Type        string      `json:"type"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Strict      *bool       `json:"strict,omitempty"`
}

// UnmarshalJSON accepts both the flat Responses shape and the nested Chat
// Completions shape, so a client that sends either to /v1/responses works.
func (t *ResponsesTool) UnmarshalJSON(b []byte) error {
	type flat ResponsesTool
	var f flat
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	var nested struct {
		Function *struct {
			Name        string      `json:"name"`
			Description string      `json:"description"`
			Parameters  interface{} `json:"parameters"`
			Strict      *bool       `json:"strict"`
		} `json:"function"`
	}
	if f.Name == "" && json.Unmarshal(b, &nested) == nil && nested.Function != nil {
		f.Name, f.Description, f.Parameters, f.Strict = nested.Function.Name, nested.Function.Description, nested.Function.Parameters, nested.Function.Strict
	}
	*t = ResponsesTool(f)
	return nil
}

// ToolsFromOpenAI converts Chat Completions tools to the flat Responses shape.
func ToolsFromOpenAI(in []OpenAITool) []ResponsesTool {
	if len(in) == 0 {
		return nil
	}
	out := make([]ResponsesTool, 0, len(in))
	for _, t := range in {
		typ := t.Type
		if typ == "" {
			typ = "function"
		}
		out = append(out, ResponsesTool{Type: typ, Name: t.Function.Name, Description: t.Function.Description, Parameters: t.Function.Parameters})
	}
	return out
}

// ToolsToOpenAI converts flat Responses tools to the nested Chat Completions shape.
func ToolsToOpenAI(in []ResponsesTool) []OpenAITool {
	if len(in) == 0 {
		return nil
	}
	out := make([]OpenAITool, 0, len(in))
	for _, t := range in {
		var ot OpenAITool
		ot.Type = "function"
		ot.Function.Name, ot.Function.Description, ot.Function.Parameters = t.Name, t.Description, t.Parameters
		out = append(out, ot)
	}
	return out
}

// ResponsesReasoning is the reasoning-config block. `effort` is one of
// `minimal|low|medium|high|xhigh`; `summary` is one of `auto|detailed|omitted`.
// Both fields are optional individually.
type ResponsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type ResponsesObject struct {
	ID                 string               `json:"id"`
	Object             string               `json:"object"`
	CreatedAt          int64                `json:"created_at"`
	Status             string               `json:"status"`
	Model              string               `json:"model"`
	Output             []ResponseOutputItem `json:"output"`
	Usage              ResponsesUsage       `json:"usage"`
	PreviousResponseID string               `json:"previous_response_id,omitempty"`
	Metadata           map[string]string    `json:"metadata,omitempty"`
	Error              *ResponsesError      `json:"error,omitempty"`
	Instructions       string               `json:"instructions,omitempty"`
	StoredInput        json.RawMessage      `json:"-"`
	StoredInstr        string               `json:"-"`
	StoredAt           int64                `json:"stored_at,omitempty"`
}

type ResponseOutputItem struct {
	ID        string                `json:"id"`
	Type      string                `json:"type"`
	Role      string                `json:"role,omitempty"`
	Status    string                `json:"status,omitempty"`
	Content   []ResponseContentPart `json:"content,omitempty"`
	CallID    string                `json:"call_id,omitempty"`
	Name      string                `json:"name,omitempty"`
	Arguments string                `json:"arguments,omitempty"`
}

type ResponseContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ResponsesUsage struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	TotalTokens       int `json:"total_tokens"`
	InputTokensDetail struct {
		// Prompt-cache hits, as the Responses API reports them.
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

type ResponsesError struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}
