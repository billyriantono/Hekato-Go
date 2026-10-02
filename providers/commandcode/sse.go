package commandcode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"strings"
	"sync"
	"time"
)

const maxStreamBytes = 4 * 1024 * 1024

// streamState holds the open-stream bookkeeping for a CommandCode NDJSON
// stream: chat.completion chunk id/model/created, running per-tool argument
// buffers, and the usage totals reported at the end.
//
// State lives across multiple NDJSON lines so deltas emitted under
// "tool-input-delta" accumulate until the matching "tool-input-end" or
// "tool-call" event flushes a complete ToolUse into the callback.
type streamState struct {
	cb *providers.StreamCallback

	id      string
	created int64
	model   string

	chunkIndex int
	openText   bool

	toolIndex     int
	toolIndexByID map[string]int
	openTools     map[string]bool
	argsBuf       map[string]*strings.Builder
	nameBuf       map[string]string

	finishReason string
	usagePrompt  int
	usageOutput  int
	usageTotal   int
	usageSet     bool

	completed bool
	mu        sync.Mutex
}

func newStreamState(cb *providers.StreamCallback) *streamState {
	if cb == nil {
		cb = &providers.StreamCallback{}
	}
	return &streamState{
		cb:            cb,
		id:            fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		created:       time.Now().Unix(),
		model:         "commandcode",
		toolIndexByID: map[string]int{},
		openTools:     map[string]bool{},
		argsBuf:       map[string]*strings.Builder{},
		nameBuf:       map[string]string{},
	}
}

// consumeNDJSON walks the upstream NDJSON stream and dispatches each line to
// the appropriate streamState hook.
func consumeNDJSON(body io.Reader, callback *providers.StreamCallback) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxStreamBytes)
	st := newStreamState(callback)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		st.dispatchLine(line)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("commandcode stream read: %w", err)
	}
	st.finish()
	return nil
}

// dispatchLine parses a single NDJSON line and forwards it to the matching
// streamState handler. Returns are intentionally void — error events route
// through the callback's OnError hook rather than terminating the scanner
// (the upstream sometimes sends `error` events alongside recoverable deltas).
func (s *streamState) dispatchLine(line []byte) {
	trimmed := bytes.TrimSpace(line)
	if bytes.HasPrefix(trimmed, []byte("data:")) {
		trimmed = bytes.TrimSpace(trimmed[len("data:"):])
	}
	if len(trimmed) == 0 {
		return
	}
	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return
	}
	var evt map[string]interface{}
	if err := json.Unmarshal(trimmed, &evt); err != nil {
		return
	}
	t, _ := evt["type"].(string)
	switch t {
	case "start", "start-step", "reasoning-start", "reasoning-end",
		"text-start", "text-end", "tool-input-end",
		"provider-metadata", "message-metadata":
		return
	case "text-delta":
		s.handleTextDelta(evt)
	case "reasoning-delta":
		s.handleReasoningDelta(evt)
	case "tool-input-start":
		s.handleToolInputStart(evt)
	case "tool-input-delta":
		s.handleToolInputDelta(evt)
	case "tool-call":
		s.handleToolCall(evt)
	case "finish-step":
		s.handleFinishStep(evt)
	case "finish":
		s.handleFinish(evt)
	case "error":
		s.raiseError(evt)
	}
}

func (s *streamState) raiseError(evt map[string]interface{}) {
	msg, _ := evt["error"].(string)
	if msg == "" {
		if m, ok := evt["error"].(map[string]interface{}); ok {
			if mm, ok := m["message"].(string); ok {
				msg = mm
			}
		}
	}
	if msg == "" {
		msg = "commandcode upstream error"
	}
	if s.cb.OnError != nil {
		s.cb.OnError(fmt.Errorf("[CommandCode error: %s]", msg))
	}
}

func (s *streamState) handleTextDelta(evt map[string]interface{}) {
	text, _ := evt["text"].(string)
	if text == "" {
		text, _ = evt["delta"].(string)
	}
	if text == "" {
		return
	}
	s.openText = true
	if s.cb.OnText != nil {
		s.cb.OnText(text, false)
	}
	s.chunkIndex++
}

func (s *streamState) handleReasoningDelta(evt map[string]interface{}) {
	text, _ := evt["text"].(string)
	if text == "" {
		return
	}
	if s.cb.OnText != nil {
		s.cb.OnText(text, true)
	}
	s.chunkIndex++
}

func (s *streamState) handleToolInputStart(evt map[string]interface{}) {
	id := pickStr(evt, "id", "toolCallId")
	if id == "" {
		id = fallbackToolID(s.toolIndex)
	}
	if _, exists := s.toolIndexByID[id]; !exists {
		s.toolIndexByID[id] = s.toolIndex
		s.toolIndex++
	}
	s.openTools[id] = true
	if _, ok := s.argsBuf[id]; !ok {
		s.argsBuf[id] = &strings.Builder{}
	}
	s.nameBuf[id] = pickStr(evt, "toolName")
	s.flushToolIfReady(id)
}

func (s *streamState) handleToolInputDelta(evt map[string]interface{}) {
	id := pickStr(evt, "id", "toolCallId")
	if id == "" {
		return
	}
	buf, ok := s.argsBuf[id]
	if !ok {
		buf = &strings.Builder{}
		s.argsBuf[id] = buf
	}
	delta, _ := evt["delta"].(string)
	if delta == "" {
		delta, _ = evt["inputTextDelta"].(string)
	}
	if delta != "" {
		buf.WriteString(delta)
	}
	s.flushToolIfReady(id)
}

func (s *streamState) handleToolCall(evt map[string]interface{}) {
	id := pickStr(evt, "toolCallId")
	if id == "" {
		return
	}
	if _, exists := s.toolIndexByID[id]; !exists {
		s.toolIndexByID[id] = s.toolIndex
		s.toolIndex++
	}
	s.openTools[id] = true
	if _, ok := s.argsBuf[id]; !ok {
		s.argsBuf[id] = &strings.Builder{}
	}
	if name := pickStr(evt, "toolName"); name != "" {
		s.nameBuf[id] = name
	}
	switch v := evt["input"].(type) {
	case string:
		s.argsBuf[id] = &strings.Builder{}
		s.argsBuf[id].WriteString(v)
	case map[string]interface{}:
		b, _ := json.Marshal(v)
		s.argsBuf[id] = &strings.Builder{}
		s.argsBuf[id].Write(b)
	}
	s.flushToolIfReady(id)
}

// flushToolIfReady emits a ToolUse once the tool's name and arguments are
// both known. Idempotent: a given id only flushes once per stream.
func (s *streamState) flushToolIfReady(id string) {
	if !s.openTools[id] {
		return
	}
	name := s.nameBuf[id]
	argsRaw := ""
	if buf, ok := s.argsBuf[id]; ok && buf != nil {
		argsRaw = strings.TrimSpace(buf.String())
	}
	if name == "" || argsRaw == "" {
		return
	}
	var input map[string]interface{}
	if argsRaw != "" {
		_ = json.Unmarshal([]byte(argsRaw), &input)
	}
	if input == nil {
		input = map[string]interface{}{}
	}
	if s.cb.OnToolUse != nil {
		s.cb.OnToolUse(providers.ToolUse{
			ToolUseID: id,
			Name:      name,
			Input:     input,
		})
	}
	delete(s.openTools, id)
}

func (s *streamState) handleFinishStep(evt map[string]interface{}) {
	if r, ok := evt["finishReason"].(string); ok {
		s.finishReason = mapFinishReason(r)
	}
	if usage, ok := evt["usage"].(map[string]interface{}); ok {
		s.captureUsage(usage)
	}
}

func (s *streamState) handleFinish(evt map[string]interface{}) {
	if r, ok := evt["finishReason"].(string); ok {
		s.finishReason = mapFinishReason(r)
	}
	if usage, ok := evt["totalUsage"].(map[string]interface{}); ok {
		s.captureUsage(usage)
	} else if usage, ok := evt["usage"].(map[string]interface{}); ok {
		s.captureUsage(usage)
	}
	for id := range s.openTools {
		s.flushToolIfReady(id)
	}
	s.completed = true
}

// finish emits the completion hooks once after the scanner loop has drained.
func (s *streamState) finish() {
	if !s.completed && s.finishReason == "" {
		s.finishReason = "stop"
	}
	if s.cb.OnComplete != nil {
		s.cb.OnComplete(s.usagePrompt, s.usageOutput)
	}
	if s.cb.OnStopReason != nil && s.finishReason != "" {
		s.cb.OnStopReason(s.finishReason)
	}
}

func (s *streamState) captureUsage(u map[string]interface{}) {
	if v, ok := providers.ReadTokenNumber(u,
		"promptTokens", "prompt_tokens", "inputTokens", "input_tokens"); ok {
		s.usagePrompt = v
		s.usageSet = true
	}
	if v, ok := providers.ReadTokenNumber(u,
		"completionTokens", "completion_tokens", "outputTokens", "output_tokens"); ok {
		s.usageOutput = v
		s.usageSet = true
	}
	if v, ok := providers.ReadTokenNumber(u, "totalTokens", "total_tokens"); ok {
		s.usageTotal = v
	}
	if read, write, ok := providers.CacheSplitFromMap(u); ok && s.cb.OnCacheUsage != nil {
		s.cb.OnCacheUsage(read, write)
	}
}

func mapFinishReason(r string) string {
	switch strings.ToLower(r) {
	case "stop", "end_turn", "finished", "complete":
		return "stop"
	case "tool_use", "tool_calls", "tool":
		return "tool_use"
	case "length", "max_tokens", "max_output_tokens":
		return "max_tokens"
	case "content_filter", "safety":
		return "content_filter"
	}
	if r == "" {
		return "stop"
	}
	return strings.ToLower(r)
}

func fallbackToolID(idx int) string {
	return "toolu_" + fmtInt(idx)
}

func pickStr(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// fmtInt is a tiny int-to-string helper to avoid importing strconv just for
// this fallback ID.
func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// logBytes is used by tests/debug to dump the wire frame for a single line.
func logBytes(prefix string, b []byte) {
	logger.Debugf("[CommandCode] %s %s", prefix, string(b))
}
