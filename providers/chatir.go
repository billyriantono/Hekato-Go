package providers

// ChatIR is the provider-neutral intermediate representation of a chat request.
// Frontend formats (Claude, OpenAI) are parsed INTO a ChatIR; provider packages
// serialize FROM it into their own wire format. This is what lets a new provider
// avoid converting from another provider's payload shape (e.g. KiroPayload).
//
// Kiro is the deliberate exception: its wire format IS its IR, so it keeps its
// own ClaudeToKiro/OpenAIToKiro converters. Every OTHER provider consumes ChatIR.
type ChatIR struct {
	Model       string
	System      string // already processed (prompt filters / thinking injection applied by the parser)
	Messages    []IRMessage
	Tools       []IRTool
	MaxTokens   int
	Temperature float64
	TopP        float64
}

// IRMessage is one conversation turn. Role is "user" or "assistant".
// ToolCalls appear on assistant turns; ToolResults on user turns.
type IRMessage struct {
	Role        string
	Text        string
	Images      []Image
	ToolCalls   []ToolUse    // assistant turn: tools the model invoked
	ToolResults []ToolResult // user turn: results answering a prior tool call
}

// IRTool is a tool definition in neutral form (no provider-specific name
// sanitization or schema cleaning — each serializer applies its own).
type IRTool struct {
	Name        string
	Description string
	Schema      any
}
