package providers

// NeutralChat is the provider-neutral intermediate representation of a chat request.
// Frontend formats (Claude, OpenAI) are parsed INTO a NeutralChat; provider packages
// serialize FROM it into their own wire format. This is what lets a new provider
// avoid converting from another provider's payload shape (e.g. KiroPayload).
//
// Kiro is the deliberate exception: its wire format IS its neutral form, so it keeps its
// own ClaudeToKiro/OpenAIToKiro converters. Every OTHER provider consumes NeutralChat.
type NeutralChat struct {
	Model        string
	SystemPrompt string // already processed (prompt filters / thinking injection applied by the parser)
	Messages     []NeutralMessage
	Tools        []NeutralTool
	MaxTokens    int
	Temperature  float64
	TopP         float64
}

// NeutralMessage is one conversation turn. Role is "user" or "assistant".
// ToolCalls appear on assistant turns; ToolResults on user turns.
type NeutralMessage struct {
	Role        string
	Text        string
	Images      []Image
	ToolCalls   []ToolUse    // assistant turn: tools the model invoked
	ToolResults []ToolResult // user turn: results answering a prior tool call
}

// NeutralTool is a tool definition in neutral form (no provider-specific name
// sanitization or schema cleaning — each serializer applies its own).
type NeutralTool struct {
	Name        string
	Description string
	InputSchema any
}
