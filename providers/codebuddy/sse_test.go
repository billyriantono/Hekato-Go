package codebuddy

import (
	"errors"
	"strings"
	"testing"

	"hekato-go/providers"
)

// Captured from CodeBuddy Global for gpt-5.6-sol on 2026-10-02: HTTP 200, one
// empty chunk, finish_reason "error".
const errorStopSSE = `data: {"model":"gpt-5.6-sol","choices":[{"index":0,"delta":{"role":"assistant","content":"","reasoning_content":"","tool_calls":[]},"finish_reason":"error"}],"usage":{"prompt_tokens":0,"completion_tokens":0}}

data: [DONE]
`

func TestParseCodeBuddySSEErrorStop(t *testing.T) {
	err := parseCodeBuddySSE(strings.NewReader(errorStopSSE), nil)
	var ue *providers.UpstreamError
	if !errors.As(err, &ue) || ue.Status != 502 {
		t.Fatalf("empty error stop must fail as an upstream 502, got %v", err)
	}

	partial := `data: {"choices":[{"delta":{"content":"Hi"},"finish_reason":""}]}` + "\n" + errorStopSSE
	var text string
	cb := &providers.StreamCallback{OnText: func(s string, _ bool) { text += s }}
	if err := parseCodeBuddySSE(strings.NewReader(partial), cb); err != nil || text != "Hi" {
		t.Fatalf("partial answer must be kept: err=%v text=%q", err, text)
	}
}
