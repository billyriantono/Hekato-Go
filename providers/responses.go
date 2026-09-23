package providers

import "encoding/json"

type ResponsesRequest struct {
	Model              string            `json:"model"`
	Input              json.RawMessage   `json:"input"`
	Instructions       string            `json:"instructions,omitempty"`
	Stream             bool              `json:"stream,omitempty"`
	Tools              []OpenAITool      `json:"tools,omitempty"`
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
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type ResponsesError struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}
