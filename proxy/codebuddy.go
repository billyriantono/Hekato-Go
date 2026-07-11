package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kiro-go/config"
	"kiro-go/logger"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	codeBuddyClientVersion = "2.52.0"
	codeBuddyAppVersion    = "4.9.29177644"
	codeBuddyIDEName       = "VSCode"
	codeBuddyIDEType       = "VSCode"
	codeBuddyIDEVersion    = "1.119.0"
	codeBuddyEnvID         = "production"
	codeBuddyUserAgent     = codeBuddyIDEName + "/" + codeBuddyIDEVersion + " CodeBuddy/" + codeBuddyAppVersion
)

type codeBuddyVariant struct {
	Name   string
	Base   string
	Domain string
}

var (
	codeBuddyGlobal = codeBuddyVariant{Name: "codebuddy", Base: "https://www.codebuddy.ai", Domain: "www.codebuddy.ai"}
	codeBuddyCN     = codeBuddyVariant{Name: "codebuddy-cn", Base: "https://copilot.tencent.com", Domain: "www.codebuddy.cn"}
)

type codeBuddyModel struct {
	ID      string
	OwnedBy string
	Image   bool
}

var codeBuddyGlobalModels = []codeBuddyModel{
	{ID: "gemini-3.1-pro", OwnedBy: "google"},
	{ID: "gemini-3.1-flash-lite", OwnedBy: "google"},
	{ID: "gemini-3.0-flash", OwnedBy: "google"},
	{ID: "gemini-2.5-pro", OwnedBy: "google"},
	{ID: "gemini-2.5-flash", OwnedBy: "google"},
	{ID: "gpt-5.5", OwnedBy: "openai"},
	{ID: "gpt-5.4", OwnedBy: "openai"},
	{ID: "gpt-5.2", OwnedBy: "openai"},
	{ID: "gpt-5.3-codex", OwnedBy: "openai"},
	{ID: "gpt-5.2-codex", OwnedBy: "openai"},
	{ID: "gpt-5.1", OwnedBy: "openai"},
	{ID: "gpt-5.1-codex", OwnedBy: "openai"},
	{ID: "gpt-5.1-codex-max", OwnedBy: "openai"},
	{ID: "gpt-5.1-codex-mini", OwnedBy: "openai"},
	{ID: "deepseek-v3-2-volc", OwnedBy: "deepseek"},
	{ID: "claude-opus-4.6", OwnedBy: "anthropic"},
	{ID: "claude-opus-4.7-1m", OwnedBy: "anthropic"},
	{ID: "kimi-k2.5", OwnedBy: "moonshot"},
}

var codeBuddyCNModels = []codeBuddyModel{
	{ID: "auto", OwnedBy: "enowxlabs"},
	{ID: "glm-5.2", OwnedBy: "zhipu"},
	{ID: "glm-5.1", OwnedBy: "zhipu"},
	{ID: "glm-5.0", OwnedBy: "zhipu"},
	{ID: "glm-5.0-turbo", OwnedBy: "zhipu"},
	{ID: "glm-5v-turbo", OwnedBy: "zhipu"},
	{ID: "glm-4.7", OwnedBy: "zhipu"},
	{ID: "glm-4.6", OwnedBy: "zhipu"},
	{ID: "glm-4.6v", OwnedBy: "zhipu"},
	{ID: "hunyuan-image-v3.0", OwnedBy: "tencent", Image: true},
	{ID: "deepseek-v4-pro", OwnedBy: "deepseek"},
	{ID: "deepseek-v4-flash", OwnedBy: "deepseek"},
	{ID: "deepseek-r1", OwnedBy: "deepseek"},
	{ID: "kimi-k2.7", OwnedBy: "moonshot"},
	{ID: "kimi-k2.6", OwnedBy: "moonshot"},
	{ID: "kimi-k2.5", OwnedBy: "moonshot"},
	{ID: "minimax-m3", OwnedBy: "minimax"},
	{ID: "minimax-m2.7", OwnedBy: "minimax"},
	{ID: "hy3-preview", OwnedBy: "tencent"},
	{ID: "claude-haiku-4.5", OwnedBy: "anthropic"},
}

func isCodeBuddyAccount(account *config.Account) bool {
	if account == nil {
		return false
	}
	am := strings.ToLower(strings.TrimSpace(account.AuthMethod))
	provider := strings.ToLower(strings.TrimSpace(account.Provider))
	return strings.Contains(am, "codebuddy") || strings.Contains(provider, "codebuddy")
}

func codeBuddyVariantForAccount(account *config.Account) codeBuddyVariant {
	if account == nil {
		return codeBuddyGlobal
	}
	joined := strings.ToLower(strings.TrimSpace(account.AuthMethod + " " + account.Provider + " " + account.Region))
	if strings.Contains(joined, "cn") || strings.Contains(joined, "china") || strings.Contains(joined, "tencent") || strings.Contains(joined, "copilot") {
		return codeBuddyCN
	}
	return codeBuddyGlobal
}

func codeBuddyToken(account *config.Account) string {
	if account == nil {
		return ""
	}
	if strings.TrimSpace(account.AccessToken) != "" {
		return strings.TrimSpace(account.AccessToken)
	}
	return strings.TrimSpace(account.RefreshToken)
}

func codeBuddyAuthHeader(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}

func applyCodeBuddyHeaders(h http.Header, v codeBuddyVariant, token string) {
	conv := codeBuddyTraceID()
	reqID := codeBuddyTraceID()
	traceID := codeBuddyTraceID()
	spanID := codeBuddyTraceID()[:16]
	parentSpanID := codeBuddyTraceID()[:16]

	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Content-Type", "application/json")
	h.Set("X-Requested-With", "XMLHttpRequest")
	h.Set("X-Conversation-ID", conv)
	h.Set("X-Conversation-Request-ID", reqID)
	h.Set("X-Conversation-Message-ID", reqID)
	h.Set("X-Request-ID", reqID)
	h.Set("X-Agent-Intent", "craft")
	h.Set("X-IDE-Type", codeBuddyIDEType)
	h.Set("X-IDE-Name", codeBuddyIDEName)
	h.Set("X-IDE-Version", codeBuddyIDEVersion)
	h.Set("X-Product-Version", codeBuddyAppVersion)
	h.Set("X-Request-Trace-Id", traceID)
	h.Set("X-Env-ID", codeBuddyEnvID)
	h.Set("X-Domain", v.Domain)
	h.Set("X-Product", "SaaS")
	h.Set("User-Agent", codeBuddyUserAgent)
	h.Set("Authorization", codeBuddyAuthHeader(token))
	// API-key mode expects both Bearer auth and X-API-Key (OAuth mode uses only
	// Authorization). Supplying both matches CodeBuddy API-key clients and is safe
	// for the API-key accounts this proxy imports.
	h.Set("X-API-Key", strings.TrimSpace(token))
	h.Set("b3", traceID+"-"+spanID+"-1-"+parentSpanID)
	h.Set("X-B3-TraceId", traceID)
	h.Set("X-B3-ParentSpanId", parentSpanID)
	h.Set("X-B3-SpanId", spanID)
	h.Set("X-B3-Sampled", "1")
}

func codeBuddyTraceID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func codeBuddyModelsForAccount(account *config.Account) []ModelInfo {
	src := codeBuddyGlobalModels
	if codeBuddyVariantForAccount(account).Name == codeBuddyCN.Name {
		src = codeBuddyCNModels
	}
	out := make([]ModelInfo, 0, len(src))
	for _, m := range src {
		inputTypes := []string{"text"}
		if m.Image {
			inputTypes = append(inputTypes, "image")
		}
		out = append(out, ModelInfo{
			ModelId:    m.ID,
			ModelName:  m.ID,
			InputTypes: inputTypes,
		})
	}
	return out
}

type codeBuddyChatRequest struct {
	Model         string                 `json:"model"`
	Messages      []OpenAIMessage        `json:"messages"`
	MaxTokens     int                    `json:"max_tokens,omitempty"`
	Temperature   float64                `json:"temperature,omitempty"`
	TopP          float64                `json:"top_p,omitempty"`
	Stream        bool                   `json:"stream"`
	StreamOptions map[string]bool        `json:"stream_options,omitempty"`
	Tools         []OpenAITool           `json:"tools,omitempty"`
	Extra         map[string]interface{} `json:"-"`
}

// ClaudeToCodeBuddy converts a Claude-format request to CodeBuddy's native
// OpenAI-compatible /v2/chat/completions body. It intentionally reuses the
// existing ClaudeToKiro normalizer so prompt/tool/image behavior stays identical
// across Kiro and CodeBuddy until we split the shared normalization layer.
func ClaudeToCodeBuddy(req *ClaudeRequest, thinking bool) codeBuddyChatRequest {
	out := codeBuddyFromNormalizedPayload(ClaudeToKiro(req, thinking))
	// CodeBuddy's CLI path is streaming-first. We always request upstream SSE and
	// aggregate locally for non-stream downstream clients.
	out.Stream = true
	out.StreamOptions = map[string]bool{"include_usage": true}
	return out
}

// OpenAIToCodeBuddy converts an OpenAI Chat Completions request to CodeBuddy's
// native /v2/chat/completions body. Match enowx's CodeBuddy provider: OpenAI
// chat requests stay OpenAI-on-the-wire instead of passing through Kiro's
// payload shape first.
func OpenAIToCodeBuddy(req *OpenAIRequest, thinking bool) codeBuddyChatRequest {
	if req == nil {
		return codeBuddyChatRequest{Model: "auto", Stream: false}
	}
	out := codeBuddyChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		// CodeBuddy Global accepts the same streaming shape used by the CLI/enowx
		// common path. Non-stream client responses are aggregated by our handler.
		Stream:        true,
		StreamOptions: map[string]bool{"include_usage": true},
		Tools:         req.Tools,
	}
	if len(out.Messages) == 0 {
		out.Messages = []OpenAIMessage{{Role: "user", Content: minimalFallbackUserContent}}
	}
	out.Messages = ensureCodeBuddySystemMessage(out.Messages)
	return out
}

func codeBuddyFromNormalizedPayload(payload *KiroPayload) codeBuddyChatRequest {
	model := "auto"
	if payload != nil {
		if m := strings.TrimSpace(payload.ConversationState.CurrentMessage.UserInputMessage.ModelID); m != "" {
			model = m
		}
	}

	req := codeBuddyChatRequest{
		Model:  model,
		Stream: true,
	}
	if payload == nil {
		return req
	}
	if payload.InferenceConfig != nil {
		req.MaxTokens = payload.InferenceConfig.MaxTokens
		req.Temperature = payload.InferenceConfig.Temperature
		req.TopP = payload.InferenceConfig.TopP
	}

	for _, h := range payload.ConversationState.History {
		appendKiroHistoryAsOpenAI(&req.Messages, h)
	}
	current := payload.ConversationState.CurrentMessage.UserInputMessage
	appendKiroUserAsOpenAI(&req.Messages, current)
	if current.UserInputMessageContext != nil {
		req.Tools = convertKiroToolsToOpenAI(current.UserInputMessageContext.Tools)
	}
	if len(req.Messages) == 0 {
		req.Messages = append(req.Messages, OpenAIMessage{Role: "user", Content: minimalFallbackUserContent})
	}
	req.Messages = ensureCodeBuddySystemMessage(req.Messages)
	return req
}

func ensureCodeBuddySystemMessage(messages []OpenAIMessage) []OpenAIMessage {
	for _, msg := range messages {
		if strings.EqualFold(strings.TrimSpace(msg.Role), "system") {
			return messages
		}
	}
	out := make([]OpenAIMessage, 0, len(messages)+1)
	out = append(out, OpenAIMessage{Role: "system", Content: "You are CodeBuddy, a helpful AI coding assistant."})
	out = append(out, messages...)
	return out
}

func appendKiroHistoryAsOpenAI(messages *[]OpenAIMessage, h KiroHistoryMessage) {
	if h.UserInputMessage != nil {
		appendKiroUserAsOpenAI(messages, *h.UserInputMessage)
	}
	if h.AssistantResponseMessage != nil {
		msg := OpenAIMessage{Role: "assistant", Content: h.AssistantResponseMessage.Content}
		for _, tu := range h.AssistantResponseMessage.ToolUses {
			msg.ToolCalls = append(msg.ToolCalls, kiroToolUseToOpenAI(tu))
		}
		*messages = append(*messages, msg)
	}
}

func appendKiroUserAsOpenAI(messages *[]OpenAIMessage, u KiroUserInputMessage) {
	if u.UserInputMessageContext != nil {
		for _, tr := range u.UserInputMessageContext.ToolResults {
			*messages = append(*messages, OpenAIMessage{
				Role:       "tool",
				ToolCallID: tr.ToolUseID,
				Content:    kiroToolResultText(tr),
			})
		}
	}
	content := strings.TrimSpace(u.Content)
	if content == "" || strings.HasPrefix(content, toolResultsContinuationPrefix) {
		return
	}
	*messages = append(*messages, OpenAIMessage{Role: "user", Content: codeBuddyContent(u.Content, u.Images)})
}

func codeBuddyContent(text string, images []KiroImage) interface{} {
	if len(images) == 0 {
		return text
	}
	parts := make([]map[string]interface{}, 0, len(images)+1)
	if strings.TrimSpace(text) != "" {
		parts = append(parts, map[string]interface{}{"type": "text", "text": text})
	}
	for _, img := range images {
		format := strings.TrimSpace(img.Format)
		if format == "" {
			format = "png"
		}
		parts = append(parts, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": "data:image/" + format + ";base64," + img.Source.Bytes,
			},
		})
	}
	return parts
}

func kiroToolResultText(tr KiroToolResult) string {
	parts := make([]string, 0, len(tr.Content))
	for _, c := range tr.Content {
		if strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func kiroToolUseToOpenAI(tu KiroToolUse) ToolCall {
	args, _ := json.Marshal(tu.Input)
	tc := ToolCall{ID: tu.ToolUseID, Type: "function"}
	tc.Function.Name = tu.Name
	tc.Function.Arguments = string(args)
	return tc
}

func convertKiroToolsToOpenAI(tools []KiroToolWrapper) []OpenAITool {
	out := make([]OpenAITool, 0, len(tools))
	for _, kt := range tools {
		var ot OpenAITool
		ot.Type = "function"
		ot.Function.Name = kt.ToolSpecification.Name
		ot.Function.Description = kt.ToolSpecification.Description
		ot.Function.Parameters = kt.ToolSpecification.InputSchema.JSON
		out = append(out, ot)
	}
	return out
}

func callCodeBuddyChatAPI(account *config.Account, payload *KiroPayload, callback *KiroStreamCallback) error {
	return callCodeBuddyChatRequestAPI(account, codeBuddyFromNormalizedPayload(payload), callback)
}

func callCodeBuddyChatRequestAPI(account *config.Account, chatReq codeBuddyChatRequest, callback *KiroStreamCallback) error {
	token := codeBuddyToken(account)
	if token == "" {
		return fmt.Errorf("codebuddy: api key is required")
	}
	v := codeBuddyVariantForAccount(account)

	resp, rawErr, err := doCodeBuddyChatRequest(account, v, token, chatReq)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("codebuddy: empty response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := rawErr
		if len(body) == 0 {
			body, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		}
		return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, v.Name, string(body))
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/event-stream") {
		return parseCodeBuddySSE(resp.Body, callback)
	}
	return parseCodeBuddyJSON(resp.Body, callback)
}

func doCodeBuddyChatRequest(account *config.Account, v codeBuddyVariant, token string, chatReq codeBuddyChatRequest) (*http.Response, []byte, error) {
	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, nil, err
	}
	logger.Debugf("[CodeBuddy] Request to %s model=%s stream=%t body=%s", v.Name, chatReq.Model, chatReq.Stream, string(reqBody))

	req, err := http.NewRequest(http.MethodPost, v.Base+"/v2/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, err
	}
	applyCodeBuddyHeaders(req.Header, v, token)
	if strings.TrimSpace(chatReq.Model) != "" {
		req.Header.Set("X-Model-ID", strings.TrimSpace(chatReq.Model))
	}

	resp, err := GetClientForProxy(ResolveAccountProxyURL(account)).Do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode == http.StatusOK {
		return resp, nil, nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp, body, nil
}

func shouldRetryCodeBuddyAuto(status int, body []byte, model string) bool {
	if status != http.StatusBadRequest || strings.EqualFold(strings.TrimSpace(model), "auto") {
		return false
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "11101") || strings.Contains(lower, "invalid request") || strings.Contains(lower, "parse message failed")
}

type codeBuddyToolDeltaState struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func parseCodeBuddySSE(body io.Reader, callback *KiroStreamCallback) error {
	if callback == nil {
		callback = &KiroStreamCallback{}
	}
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	toolStates := map[int]*codeBuddyToolDeltaState{}
	var inputTokens, outputTokens int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var evt map[string]interface{}
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}
		if usage, ok := evt["usage"].(map[string]interface{}); ok {
			if v, ok := readTokenNumber(usage, "prompt_tokens", "promptTokens", "input_tokens", "inputTokens"); ok {
				inputTokens = v
			}
			if v, ok := readTokenNumber(usage, "completion_tokens", "completionTokens", "output_tokens", "outputTokens"); ok {
				outputTokens = v
			}
		}
		choices, _ := evt["choices"].([]interface{})
		for _, rawChoice := range choices {
			choice, _ := rawChoice.(map[string]interface{})
			if choice == nil {
				continue
			}
			if delta, _ := choice["delta"].(map[string]interface{}); delta != nil {
				dispatchCodeBuddyDelta(delta, toolStates, callback)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	flushCodeBuddyTools(toolStates, callback)
	if callback.OnComplete != nil {
		callback.OnComplete(inputTokens, outputTokens)
	}
	return nil
}

func dispatchCodeBuddyDelta(delta map[string]interface{}, toolStates map[int]*codeBuddyToolDeltaState, callback *KiroStreamCallback) {
	if text, ok := delta["content"].(string); ok && text != "" && callback.OnText != nil {
		callback.OnText(text, false)
	}
	for _, key := range []string{"reasoning_content", "reasoning", "reasoningContent"} {
		if text, ok := delta[key].(string); ok && text != "" && callback.OnText != nil {
			callback.OnText(text, true)
		}
	}
	calls, _ := delta["tool_calls"].([]interface{})
	for _, rawCall := range calls {
		call, _ := rawCall.(map[string]interface{})
		if call == nil {
			continue
		}
		idx := len(toolStates)
		if f, ok := call["index"].(float64); ok {
			idx = int(f)
		}
		st := toolStates[idx]
		if st == nil {
			st = &codeBuddyToolDeltaState{}
			toolStates[idx] = st
		}
		if id, ok := call["id"].(string); ok && id != "" {
			st.ID = id
		}
		if fn, _ := call["function"].(map[string]interface{}); fn != nil {
			if name, ok := fn["name"].(string); ok && name != "" {
				st.Name = name
			}
			if args, ok := fn["arguments"].(string); ok && args != "" {
				st.Arguments.WriteString(args)
			}
		}
	}
}

func flushCodeBuddyTools(toolStates map[int]*codeBuddyToolDeltaState, callback *KiroStreamCallback) {
	if callback == nil || callback.OnToolUse == nil {
		return
	}
	for _, st := range toolStates {
		if st == nil || st.Name == "" {
			continue
		}
		id := st.ID
		if id == "" {
			id = "toolu_" + uuid.New().String()
		}
		var input map[string]interface{}
		if args := strings.TrimSpace(st.Arguments.String()); args != "" {
			_ = json.Unmarshal([]byte(args), &input)
		}
		if input == nil {
			input = map[string]interface{}{}
		}
		callback.OnToolUse(KiroToolUse{ToolUseID: id, Name: st.Name, Input: input})
	}
}

func parseCodeBuddyJSON(body io.Reader, callback *KiroStreamCallback) error {
	if callback == nil {
		callback = &KiroStreamCallback{}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content          string     `json:"content"`
				ReasoningContent string     `json:"reasoning_content"`
				ToolCalls        []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage map[string]interface{} `json:"usage"`
	}
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return err
	}
	for _, c := range out.Choices {
		if c.Message.ReasoningContent != "" && callback.OnText != nil {
			callback.OnText(c.Message.ReasoningContent, true)
		}
		if c.Message.Content != "" && callback.OnText != nil {
			callback.OnText(c.Message.Content, false)
		}
		if callback.OnToolUse != nil {
			for _, tc := range c.Message.ToolCalls {
				var input map[string]interface{}
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				if input == nil {
					input = map[string]interface{}{}
				}
				callback.OnToolUse(KiroToolUse{ToolUseID: tc.ID, Name: tc.Function.Name, Input: input})
			}
		}
	}
	if callback.OnComplete != nil {
		inTok, _ := readTokenNumber(out.Usage, "prompt_tokens", "promptTokens", "input_tokens", "inputTokens")
		outTok, _ := readTokenNumber(out.Usage, "completion_tokens", "completionTokens", "output_tokens", "outputTokens")
		callback.OnComplete(inTok, outTok)
	}
	return nil
}

const codeBuddyCNMainSubProduct = "sp_tcaca_codebuddy_ide"

func FetchCodeBuddyUsage(account *config.Account) (*config.AccountInfo, error) {
	info := &config.AccountInfo{LastRefresh: time.Now().Unix()}
	if codeBuddyVariantForAccount(account).Name != codeBuddyCN.Name {
		info.SubscriptionType = "CODEBUDDY"
		info.SubscriptionTitle = "CodeBuddy Global"
		return info, nil
	}
	limit, used, remain, err := fetchCodeBuddyCNCredits(account)
	if err != nil {
		return nil, err
	}
	info.SubscriptionType = "CODEBUDDY_CN"
	info.SubscriptionTitle = "CodeBuddy China"
	info.UsageCurrent = used
	info.UsageLimit = limit
	if limit > 0 {
		info.UsagePercent = used / limit
	}
	_ = remain
	return info, nil
}

func fetchCodeBuddyCNCredits(account *config.Account) (limit, used, remain float64, err error) {
	token := codeBuddyToken(account)
	if token == "" {
		return 0, 0, 0, fmt.Errorf("codebuddy-cn: api key is required")
	}
	v := codeBuddyVariantForAccount(account)
	now := time.Now()
	body, _ := json.Marshal(map[string]interface{}{
		"PageNumber":               1,
		"PageSize":                 100,
		"ProductCode":              "p_tcaca",
		"Status":                   []int{0, 3},
		"PackageEndTimeRangeBegin": now.Add(-24 * time.Hour).Format("2006-01-02T15:04:05Z"),
		"PackageEndTimeRangeEnd":   now.Add(365 * 24 * time.Hour).Format("2006-01-02T15:04:05Z"),
	})
	req, err := http.NewRequest(http.MethodPost, v.Base+"/v2/billing/meter/get-user-resource", bytes.NewReader(body))
	if err != nil {
		return 0, 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", codeBuddyAuthHeader(token))
	req.Header.Set("X-Domain", v.Domain)
	req.Header.Set("User-Agent", "CLI/2.106.3 CodeBuddy/2.106.3")

	resp, err := GetRestClientForProxy(ResolveAccountProxyURL(account)).Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, fmt.Errorf("codebuddy-cn credits (HTTP %d)", resp.StatusCode)
	}

	var out struct {
		Code int `json:"code"`
		Data struct {
			Response struct {
				Data struct {
					Accounts []struct {
						SubProductCode      string      `json:"SubProductCode"`
						Status              int         `json:"Status"`
						CapacitySize        float64     `json:"CapacitySize"`
						CapacityUsed        float64     `json:"CapacityUsed"`
						CapacityRemain      float64     `json:"CapacityRemain"`
						CycleCapacityUsed   interface{} `json:"CycleCapacityUsed"`
						CycleCapacityRemain interface{} `json:"CycleCapacityRemain"`
					} `json:"Accounts"`
				} `json:"Data"`
			} `json:"Response"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return 0, 0, 0, err
	}
	if out.Code != 0 {
		return 0, 0, 0, fmt.Errorf("codebuddy-cn credits (code=%d)", out.Code)
	}

	for _, a := range out.Data.Response.Data.Accounts {
		if a.Status != 0 {
			continue
		}
		cycUsed, cycRemain := codeBuddyFloat(a.CycleCapacityUsed), codeBuddyFloat(a.CycleCapacityRemain)
		if a.SubProductCode == codeBuddyCNMainSubProduct {
			limit += a.CapacitySize
			used += cycUsed
			remain += cycRemain
		} else {
			bUsed, bRemain := cycUsed, cycRemain
			if bRemain == 0 && a.CapacityRemain > 0 {
				bUsed, bRemain = a.CapacityUsed, a.CapacityRemain
			}
			limit += a.CapacitySize
			used += bUsed
			remain += bRemain
		}
	}
	return limit, used, remain, nil
}

func codeBuddyFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}
