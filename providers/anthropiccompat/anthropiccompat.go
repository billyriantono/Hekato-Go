// Package anthropiccompat implements a generic Anthropic Messages–compatible
// upstream provider. Operators configure each account with a base URL and API
// key; the package sends standard Anthropic Messages requests at
// {baseURL}/v1/messages and consumes the standard Anthropic response shape.
//
// Use cases include Anthropic Platform, Amazon Bedrock (Anthropic), Google
// Vertex AI (Anthropic), and any other vendor that exposes an
// Anthropic-Messages-compatible endpoint.
package anthropiccompat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"net/http"
	"strings"
)

// Anthropic-version header. 2023-06-01 is the oldest stable version and is
// what every Anthropic-compatible vendor accepts.
const anthropicVersion = "2023-06-01"

// BaseURL is a var (not const) so tests can point the package at httptest
// servers; messagesURL / modelsURL are computed at call time.

// MessagesRequest is the wire format for Anthropic Messages. Defined locally
// because providers does not expose it; the proxy package builds one of
// these for each request from either ClaudeRequest or NeutralChat.
type MessagesRequest struct {
	Model     string            `json:"model"`
	Messages  []MessagesMessage `json:"messages"`
	System    string            `json:"system,omitempty"`
	MaxTokens int               `json:"max_tokens"`
	Stream    bool              `json:"stream,omitempty"`
	Tools     []MessagesTool    `json:"tools,omitempty"`
	Thinking  *MessagesThinking `json:"thinking,omitempty"`
}

// MessagesMessage is one turn. Content is a string (text-only) or []ContentBlock
// (mixed text + images + tool_use + tool_result).
type MessagesMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// MessagesTool is an Anthropic tool definition. InputSchema is JSON Schema.
type MessagesTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
}

// MessagesThinking turns on extended thinking.
type MessagesThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// messagesURL appends /v1/messages to the configured base URL.
func messagesURL(base string) string {
	return strings.TrimRight(base, "/") + "/v1/messages"
}

// modelsURL appends /v1/models to the configured base URL for the model
// discovery endpoint. The Anthropic standard; most compatible vendors expose
// the same path.
func modelsURL(base string) string {
	return strings.TrimRight(base, "/") + "/v1/models"
}

// CallMessages forwards an Anthropic Messages request to the configured
// vendor. The proxy wires this in via proxy/provider_anthropiccompat.go;
// chatFromClaude passes *proxy.ClaudeRequest straight in (the request shapes
// are isomorphic), and chatFromOpenAI goes through NeutralChat first.
func CallMessages(account *config.Account, req *MessagesRequest, callback *providers.StreamCallback) error {
	if account == nil {
		return fmt.Errorf("anthropiccompat: nil account")
	}
	if account.BaseURL == "" {
		return fmt.Errorf("anthropiccompat: account %s has no base URL configured", account.ID)
	}
	if account.CompatAPIKey == "" {
		return fmt.Errorf("anthropiccompat: account %s has no API key configured", account.ID)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal anthropiccompat request: %w", err)
	}
	resp, err := doRequest(account, body, req.Stream)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return providers.Errorf(resp.StatusCode, "anthropiccompat HTTP %d: %s", resp.StatusCode, raw)
	}

	if req.Stream {
		return consumeSSE(resp.Body, callback)
	}
	return decodeNonStream(resp.Body, callback)
}

// doRequest issues the messages request. Most Anthropic vendors expect
// x-api-key (not Bearer); some accept both, so we set both for safety.
func doRequest(account *config.Account, body []byte, stream bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, messagesURL(account.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build anthropiccompat request: %w", err)
	}
	req.Header.Set("x-api-key", account.CompatAPIKey)
	req.Header.Set("Authorization", "Bearer "+account.CompatAPIKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	return providers.GetRestClientForAccount(account).Do(req)
}

// nonStreamResponse mirrors the Anthropic Messages non-stream response shape.
type nonStreamResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Content      []contentBlock `json:"content"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// contentBlock is one block in the response content array. We only consume
// text + tool_use; everything else (image, document) is ignored.
type contentBlock struct {
	Type string `json:"type"`
	// text
	Text string `json:"text,omitempty"`
	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// decodeNonStream reads a non-streaming response and emits text + tool_use
// blocks through callback.
func decodeNonStream(r io.Reader, callback *providers.StreamCallback) error {
	raw, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return fmt.Errorf("read anthropiccompat body: %w", err)
	}
	var resp nonStreamResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("decode anthropiccompat response: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("anthropiccompat: %s (%s)", resp.Error.Message, resp.Error.Type)
	}
	if callback != nil {
		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				if text := strings.TrimSpace(block.Text); text != "" && callback.OnText != nil {
					callback.OnText(text, false)
				}
			case "tool_use":
				if callback.OnToolUse != nil {
					var input map[string]interface{}
					if len(block.Input) > 0 {
						_ = json.Unmarshal(block.Input, &input)
					}
					callback.OnToolUse(providers.ToolUse{
						ToolUseID: block.ID,
						Name:      block.Name,
						Input:     input,
					})
				}
			}
		}
		if resp.StopReason != "" && callback.OnStopReason != nil {
			callback.OnStopReason(resp.StopReason)
		}
		if callback.OnComplete != nil {
			callback.OnComplete(resp.Usage.InputTokens, resp.Usage.OutputTokens)
		}
	}
	return nil
}

// consumeSSE parses an Anthropic-Messages-compatible SSE stream. Events we
// care about:
//
//	message_start:    {message: {id, model, ...}} — metadata; ignored.
//	content_block_start:  {index, content_block: {type, text|id|name|input}}
//	content_block_delta:  {index, delta: {type:"text_delta", text}|{type:"input_json_delta", partial_json}}
//	content_block_stop:   {index} — fires per block, no payload.
//	message_delta:    {delta: {stop_reason, stop_sequence}, usage: {output_tokens}}
//	message_stop:     {} — terminal.
//
// Anything we don't recognise is logged and skipped, matching the rest of
// the proxy's permissive parsers.
func consumeSSE(r io.Reader, callback *providers.StreamCallback) error {
	scanner := bufio.NewScanner(r)
	// Tool-input JSON deltas can exceed Scanner's default 64 KiB cap; expand.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	type blockState struct {
		id, name string
		input    strings.Builder
		// type is "text" or "tool_use"; other types are ignored.
		kind string
	}
	blocks := map[int]*blockState{}

	var inputTokens, outputTokens int
	var sawStop bool
	stopReason := ""

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			// event: lines are not used by the Anthropic spec; we infer from data.
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var ev struct {
			Type         string        `json:"type"`
			Index        int           `json:"index"`
			ContentBlock *contentBlock `json:"content_block,omitempty"`
			Delta        *struct {
				Type        string `json:"type"`
				Text        string `json:"text,omitempty"`
				PartialJSON string `json:"partial_json,omitempty"`
				StopReason  string `json:"stop_reason,omitempty"`
			} `json:"delta,omitempty"`
			Message *nonStreamResponse `json:"message,omitempty"`
			Usage   *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage,omitempty"`
		}
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			logger.Warnf("[anthropiccompat] malformed SSE event: %v payload=%q", err, truncate(payload))
			continue
		}

		switch ev.Type {
		case "message_start":
			if ev.Message != nil && ev.Message.Usage.InputTokens > 0 {
				inputTokens = ev.Message.Usage.InputTokens
			}
		case "content_block_start":
			if ev.ContentBlock == nil {
				continue
			}
			st := &blockState{kind: ev.ContentBlock.Type}
			if ev.ContentBlock.Type == "tool_use" {
				st.id = ev.ContentBlock.ID
				st.name = ev.ContentBlock.Name
			} else if ev.ContentBlock.Type == "text" {
				if text := ev.ContentBlock.Text; text != "" && callback != nil && callback.OnText != nil {
					callback.OnText(text, false)
				}
			}
			blocks[ev.Index] = st
		case "content_block_delta":
			if ev.Delta == nil {
				continue
			}
			st := blocks[ev.Index]
			if st == nil {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Delta.Text != "" && callback != nil && callback.OnText != nil {
					callback.OnText(ev.Delta.Text, false)
				}
			case "input_json_delta":
				st.input.WriteString(ev.Delta.PartialJSON)
			}
		case "content_block_stop":
			st := blocks[ev.Index]
			if st != nil && st.kind == "tool_use" && callback != nil && callback.OnToolUse != nil {
				raw := st.input.String()
				var input map[string]interface{}
				if raw != "" {
					if !json.Valid([]byte(raw)) {
						input = map[string]interface{}{"arguments_raw": raw}
					} else {
						_ = json.Unmarshal([]byte(raw), &input)
					}
				}
				callback.OnToolUse(providers.ToolUse{
					ToolUseID: st.id,
					Name:      st.name,
					Input:     input,
				})
			}
			delete(blocks, ev.Index)
		case "message_delta":
			if ev.Delta != nil && ev.Delta.StopReason != "" {
				stopReason = ev.Delta.StopReason
			}
			if ev.Usage != nil {
				if ev.Usage.InputTokens > 0 {
					inputTokens = ev.Usage.InputTokens
				}
				if ev.Usage.OutputTokens > 0 {
					outputTokens = ev.Usage.OutputTokens
				}
			}
		case "message_stop":
			sawStop = true
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("anthropiccompat SSE: %w", err)
	}

	if callback != nil {
		if callback.OnStopReason != nil && stopReason != "" {
			callback.OnStopReason(stopReason)
		}
		if callback.OnComplete != nil && (sawStop || inputTokens > 0 || outputTokens > 0) {
			callback.OnComplete(inputTokens, outputTokens)
		}
	}
	return nil
}

// truncate caps the bytes included in error messages / debug lines so a giant
// upstream payload does not blow up the admin log column.
func truncate(s string) string {
	const max = 512
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
