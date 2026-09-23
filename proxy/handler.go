package proxy

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"hekato-go/auth"
	"hekato-go/config"
	"hekato-go/egress"
	"hekato-go/logger"
	"hekato-go/pool"
	"hekato-go/providers"
	"hekato-go/relay"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const tokenRefreshSkewSeconds int64 = 120

// RequestLog stores details about a single API request (success or failure).
type RequestLog struct {
	Time         int64   `json:"time"`         // Unix timestamp
	Endpoint     string  `json:"endpoint"`     // claude/openai/responses
	Model        string  `json:"model"`        // Requested model
	AccountID    string  `json:"accountId"`    // Account used
	Status       string  `json:"status"`       // "success" or "error"
	Error        string  `json:"error"`        // Error message (empty on success)
	ErrorType    string  `json:"errorType"`    // Error category (empty on success)
	Tokens       int     `json:"tokens"`       // Total tokens (input+output, 0 on failure)
	OutputTokens int     `json:"outputTokens"` // Output tokens only (drives TPS)
	Credits      float64 `json:"credits"`      // Credits consumed (0 on failure)
	Duration     int64   `json:"duration"`     // Request duration in ms (first-byte → end)
	TTFTMs       int64   `json:"ttftMs"`       // Time to first token / tool_use in ms
	TPS          float64 `json:"tps"`          // Output tokens per second (0 when unmeasurable)
	UserAgent    string  `json:"userAgent"`    // Client User-Agent header (truncated)
	ClientIP     string  `json:"clientIp"`     // Client IP (honors X-Forwarded-For + X-Real-IP)
}

// logUserAgentMaxLen is the truncation ceiling for the User-Agent persisted on
// the log entry. 200 chars covers any realistic CLI / agent UA while keeping
// the dashboard column readable; longer strings get "…" appended.
const logUserAgentMaxLen = 200

const requestLogsMaxSize = 500

// Handler HTTP 处理器
type Handler struct {
	pool *pool.AccountPool
	// 运行时统计 (使用原子操作)
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	totalTokens     int64
	totalCredits    float64 // float64 需要用锁保护
	creditsMu       sync.RWMutex
	startTime       int64
	stopRefresh     chan struct{}
	stopStatsSaver  chan struct{}
	// 模型缓存
	cachedModels    []ModelInfo
	modelsCacheMu   sync.RWMutex
	modelsCacheTime int64
	promptCache     *promptCacheTracker
	affinity        *accountAffinity
	limiter         *keyLimiter
	autoRouter      *autoRouter
	metrics         *metricsCollector
	warmup          warmupState
	tokenRefreshMu  sync.Mutex
	// 请求日志 (环形缓冲区，包含成功和失败)
	requestLogs   []RequestLog
	requestLogsMu sync.RWMutex
	// 请求日志持久化：appendRequestLog 把条目丢进 pendingLogs；
	// 后台 goroutine 按 flush 周期批量写盘，停机时 drain 后再退出。
	pendingLogs  chan config.PersistedRequestLog
	stopLogSaver chan struct{}
	logSaverDone chan struct{}
}

// logMetaWriter wraps the ResponseWriter for one request with the client
// metadata (User-Agent, IP) that every log entry of that request should carry.
// Carrying it on the writer (which every handler already threads through)
// keeps concurrent requests isolated; shared Handler fields raced.
type logMetaWriter struct {
	http.ResponseWriter
	userAgent string
	clientIP  string
}

func (m *logMetaWriter) Flush() {
	if f, ok := m.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (m *logMetaWriter) Unwrap() http.ResponseWriter { return m.ResponseWriter }

// withClientLogMeta returns the writer to use for the request's handler.
func withClientLogMeta(w http.ResponseWriter, r *http.Request) http.ResponseWriter {
	return &logMetaWriter{ResponseWriter: w, userAgent: truncateUA(r.UserAgent()), clientIP: clientIPFromRequest(r)}
}

// clientLogMeta extracts the metadata bound by withClientLogMeta ("" when the
// writer was not wrapped, e.g. tests or internal calls).
func clientLogMeta(w http.ResponseWriter) (userAgent, clientIP string) {
	for w != nil {
		if m, ok := w.(*logMetaWriter); ok {
			return m.userAgent, m.clientIP
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return "", ""
		}
		w = u.Unwrap()
	}
	return "", ""
}

// truncateUA keeps User-Agent strings bounded so a malicious or buggy client
// cannot blow up the in-memory ring or the dashboard column width. 200 chars
// is more than any real CLI / agent UA we have seen.
func truncateUA(ua string) string {
	ua = strings.TrimSpace(ua)
	if len(ua) <= logUserAgentMaxLen {
		return ua
	}
	return ua[:logUserAgentMaxLen-1] + "…"
}

// clientIPFromRequest returns the best-effort client IP, honoring the
// standard proxy headers. Only the leftmost X-Forwarded-For entry is used
// (the original client); X-Real-IP is the fallback. r.RemoteAddr is the
// last resort (direct connection peer).
func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type thinkingStreamSource int

const (
	thinkingSourceUnknown thinkingStreamSource = iota
	thinkingSourceReasoningEvent
	thinkingSourceTagBlock
)

func allowReasoningSource(source *thinkingStreamSource) bool {
	if *source == thinkingSourceTagBlock {
		return false
	}
	*source = thinkingSourceReasoningEvent
	return true
}

func allowTagSource(source *thinkingStreamSource) bool {
	if *source == thinkingSourceReasoningEvent {
		return false
	}
	if *source == thinkingSourceUnknown {
		*source = thinkingSourceTagBlock
	}
	return *source == thinkingSourceTagBlock
}

func validateClaudeRequestShape(req *ClaudeRequest) string {
	if len(req.Messages) == 0 {
		return "messages must not be empty"
	}
	if msg := validateClaudeThinkingConfig(req.Thinking, req.MaxTokens); msg != "" {
		return msg
	}

	hasUserContext := false
	lastRole := ""
	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			continue
		}
		lastRole = role
		if role != "user" {
			continue
		}

		text, images, toolResults := extractClaudeUserContent(msg.Content)
		if normalizeUserContent(text, len(images) > 0) != "" || len(toolResults) > 0 {
			hasUserContext = true
		}
	}

	if lastRole == "assistant" {
		return "assistant-prefill final message is not supported; last message must be user"
	}
	if !hasUserContext {
		return "at least one non-empty user message is required"
	}
	return ""
}

func validateClaudeThinkingConfig(thinking *ClaudeThinkingConfig, maxTokens int) string {
	if thinking == nil {
		return ""
	}

	kind := strings.ToLower(strings.TrimSpace(thinking.Type))
	switch kind {
	case "enabled":
		if maxTokens == 0 {
			return "thinking.type enabled cannot be used with max_tokens=0"
		}
		if thinking.BudgetTokens <= 0 {
			return "thinking.budget_tokens is required when thinking.type is enabled"
		}
		if thinking.BudgetTokens < 1024 {
			return "thinking.budget_tokens must be at least 1024"
		}
		if maxTokens > 0 && thinking.BudgetTokens >= maxTokens {
			return "thinking.budget_tokens must be less than max_tokens"
		}
	case "adaptive":
		if thinking.BudgetTokens != 0 {
			return "thinking.budget_tokens is not supported when thinking.type is adaptive"
		}
	case "disabled":
		if thinking.BudgetTokens != 0 {
			return "thinking.budget_tokens is not supported when thinking.type is disabled"
		}
	default:
		return "thinking.type must be one of: enabled, adaptive, disabled"
	}

	display := strings.ToLower(strings.TrimSpace(thinking.Display))
	if display != "" && display != "summarized" && display != "omitted" {
		return "thinking.display must be one of: summarized, omitted"
	}
	if kind == "disabled" && display != "" {
		return "thinking.display is not supported when thinking.type is disabled"
	}

	return ""
}

type claudeThinkingResponseOptions struct {
	Format      string
	OmitDisplay bool
}

func resolveClaudeThinkingResponseOptions(thinking *ClaudeThinkingConfig, defaultFormat string) claudeThinkingResponseOptions {
	opts := claudeThinkingResponseOptions{Format: defaultFormat}
	if opts.Format == "" {
		opts.Format = "thinking"
	}
	if thinking == nil {
		return opts
	}

	display := strings.ToLower(strings.TrimSpace(thinking.Display))
	switch display {
	case "summarized":
		opts.Format = "thinking"
	case "omitted":
		opts.Format = "thinking"
		opts.OmitDisplay = true
	}

	return opts
}

func validateOpenAIRequestShape(req *OpenAIRequest) string {
	if len(req.Messages) == 0 {
		return "messages must not be empty"
	}

	hasNonSystem := false
	hasUserContext := false
	lastRole := ""
	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			continue
		}
		if role != "system" {
			hasNonSystem = true
			lastRole = role
		}

		if role != "user" {
			continue
		}
		text, images := extractOpenAIUserContent(msg.Content)
		if normalizeUserContent(text, len(images) > 0) != "" {
			hasUserContext = true
		}
	}

	if !hasNonSystem {
		return "at least one non-system message is required"
	}
	if lastRole == "assistant" {
		return "assistant-prefill final message is not supported; last message must be user or tool"
	}
	if !hasUserContext {
		return "at least one non-empty user message is required"
	}
	return ""
}

func NewHandler() *Handler {
	// 启动时应用代理配置
	applyProxyConfig(config.GetProxyURL())

	totalReq, successReq, failedReq, totalTokens, totalCredits := config.GetStats()
	h := &Handler{
		pool:            pool.GetPool(),
		totalRequests:   int64(totalReq),
		successRequests: int64(successReq),
		failedRequests:  int64(failedReq),
		totalTokens:     int64(totalTokens),
		totalCredits:    totalCredits,
		startTime:       time.Now().Unix(),
		stopRefresh:     make(chan struct{}),
		stopStatsSaver:  make(chan struct{}),
		pendingLogs:     make(chan config.PersistedRequestLog, requestLogPendingCapacity),
		stopLogSaver:    make(chan struct{}),
		logSaverDone:    make(chan struct{}),
		promptCache:     newPromptCacheTracker(defaultPromptCacheTTL),
		affinity:        newAccountAffinity(),
		limiter:         newKeyLimiter(),
		autoRouter:      newAutoRouter(),
		metrics:         newMetricsCollector(),
	}
	// Restore persisted ops metrics and keep flushing them in the background.
	h.metrics.Load(config.Metrics())
	h.autoRouter.Load(config.Blobs())
	go h.runTelemetryFlusher()
	// 从持久化存储加载最近请求日志，使重启后 /logs 与 dashboard telemetry 不再清空。
	// 容量不足 (e.g. 历史未持久化、或后端未实现 RequestLogStore) 时静默回退到内存环。
	if rs := config.RequestLogs(); rs != nil {
		if entries, err := rs.LoadRecent(requestLogsMaxSize); err == nil && len(entries) > 0 {
			// 按 Time 升序填入环形缓冲；最新 N 条保留在最尾。
			h.requestLogs = make([]RequestLog, 0, len(entries))
			for _, e := range entries {
				h.requestLogs = append(h.requestLogs, RequestLog{
					Time: e.Time, Endpoint: e.Endpoint, Model: e.Model,
					AccountID: e.AccountID, Status: e.Status,
					Error: e.Error, ErrorType: e.ErrorType,
					Tokens: e.Tokens, OutputTokens: e.OutputTokens,
					Credits: e.Credits, Duration: e.Duration,
					TTFTMs: e.TTFTMs, TPS: e.TPS,
					UserAgent: e.UserAgent, ClientIP: e.ClientIP,
				})
			}
			logger.Infof("[request-log] restored %d entries from storage", len(h.requestLogs))
		}
		go h.runRequestLogFlusher(rs)
	} else {
		// 后端不支持持久化 → 不启动 flusher，pendingLogs 保持 nil，appendRequestLog 走纯内存路径。
		close(h.logSaverDone)
	}
	// 启动后台刷新
	// Routing needs per-account model lists from the first request: restore
	// the last known live lists from storage, overlay static catalogs, then
	// fetch fresh lists in the background immediately.
	h.pool.RestoreModelLists(config.Blobs())
	h.seedStaticModelLists()
	go h.backgroundRefresh()
	// 启动后台统计保存 (每30秒保存一次)
	go h.backgroundStatsSaver()
	// 清理过期的 stored responses（>30 天）
	go purgeExpiredResponses(responsesDefaultTTL)
	return h
}

// accountRefreshInterval: how often every account's token and quota/credits
// are re-checked upstream. Dashboard setting wins, then ACCOUNT_REFRESH_MINUTES,
// then 30 minutes.
func accountRefreshInterval() time.Duration {
	if v := config.GetAccountRefreshMinutes(); v >= 1 {
		return time.Duration(v) * time.Minute
	}
	if v, err := strconv.Atoi(os.Getenv("ACCOUNT_REFRESH_MINUTES")); err == nil && v >= 1 {
		return time.Duration(v) * time.Minute
	}
	return 30 * time.Minute
}

// runTelemetryFlusher persists ops telemetry (metrics buckets, auto-router
// state) every 30s and once more on shutdown.
func (h *Handler) runTelemetryFlusher() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.metrics.Flush()
			h.autoRouter.Flush()
		case <-h.stopStatsSaver:
			h.metrics.Flush()
			h.autoRouter.Flush()
			return
		}
	}
}

// Shutdown stops background loops and flushes stats and telemetry to storage.
func (h *Handler) Shutdown() {
	close(h.stopRefresh)
	close(h.stopStatsSaver)
	h.saveStats()
	h.metrics.Flush()
	h.autoRouter.Flush()
	// Drain pending request logs to disk before exit. Non-blocking on shutdown:
	// if no flusher was started (backend without RequestLogStore), logSaverDone
	// was closed in NewHandler, so the select hits it immediately.
	if h.stopLogSaver != nil {
		close(h.stopLogSaver)
	}
	if h.logSaverDone != nil {
		<-h.logSaverDone
	}
}

// seedStaticModelLists fills the pool's per-account model lists for providers
// whose catalog is local (no network), synchronously.
func (h *Handler) seedStaticModelLists() {
	for _, acc := range config.GetEnabledAccounts() {
		adapter, err := adapterForAccount(&acc)
		if err != nil || !adapter.staticModels || adapter.listModels == nil {
			continue
		}
		models, err := adapter.listModels(&acc)
		if err != nil || len(models) == 0 {
			continue
		}
		ids := make([]string, 0, len(models))
		for _, m := range models {
			ids = append(ids, m.ModelId)
		}
		h.pool.SetModelList(acc.ID, ids)
	}
}

// backgroundRefresh 后台定时刷新账户信息
func (h *Handler) backgroundRefresh() {
	// Fetch live model lists right away (the pool was restored from the last
	// known lists + static catalogs, so routing works meanwhile); give the
	// upstreams a short grace period before the first full warmup.
	h.refreshModelsCache()
	h.pool.PersistModelLists(config.Blobs())
	time.Sleep(10 * time.Second)
	h.refreshAllAccounts()

	// Timer is re-armed every cycle so a changed interval applies without restart.
	for {
		timer := time.NewTimer(accountRefreshInterval())
		select {
		case <-timer.C:
			h.refreshModelsCache()
			h.pool.PersistModelLists(config.Blobs())
			h.refreshAllAccounts()
		case <-h.stopRefresh:
			timer.Stop()
			return
		}
	}
}

// refreshAllAccounts runs the warmup cycle (token refresh, quota, optional probe,
// auto-recovery) over every candidate account.
func (h *Handler) refreshAllAccounts() {
	h.runWarmup(nil)
}

// validateApiKey 验证 API Key（Bool 包装，旧签名仍被部分调用方使用）
func (h *Handler) validateApiKey(r *http.Request) bool {
	_, err := h.authenticate(r)
	return err == nil
}

// authenticateForClaude runs authenticate + rate limiting and writes a Claude-style
// error on failure. Returns the request with the matched API key injected into
// context plus a release func (always call it), or nil if rejected.
func (h *Handler) authenticateForClaude(w http.ResponseWriter, r *http.Request) (*http.Request, func()) {
	ar, release, ae := h.admit(r)
	if ae != nil {
		if ae.status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "1")
		}
		h.sendClaudeError(w, ae.status, ae.code, ae.message)
		return nil, nil
	}
	return ar, release
}

// authenticateForOpenAI runs authenticate + rate limiting and writes an OpenAI-style error on failure.
func (h *Handler) authenticateForOpenAI(w http.ResponseWriter, r *http.Request) (*http.Request, func()) {
	ar, release, ae := h.admit(r)
	if ae != nil {
		if ae.status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "1")
		}
		h.sendOpenAIError(w, ae.status, ae.code, ae.message)
		return nil, nil
	}
	return ar, release
}

// ServeHTTP 路由分发
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Debug-level request trace for fine-grained visibility
	logger.Debugf("[HTTP] %s %s from %s", r.Method, path, r.RemoteAddr)

	// CORS - 完整的头部支持
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key, anthropic-version, anthropic-beta, x-api-key, x-stainless-os, x-stainless-lang, x-stainless-package-version, x-stainless-runtime, x-stainless-runtime-version, x-stainless-arch")
	w.Header().Set("Access-Control-Expose-Headers", "x-request-id, x-hekato-routed-model, x-hekato-route-reason, x-ratelimit-limit-requests, x-ratelimit-limit-tokens, x-ratelimit-remaining-requests, x-ratelimit-remaining-tokens, x-ratelimit-reset-requests, x-ratelimit-reset-tokens")

	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}

	// 路由
	switch {
	// API 端点（需要验证 API Key）
	case path == "/v1/messages" || path == "/messages" || path == "/anthropic/v1/messages":
		w = withClientLogMeta(w, r)
		ar, release := h.authenticateForClaude(w, r)
		if ar == nil {
			return
		}
		defer release()
		h.handleClaudeMessages(w, ar)
	case path == "/v1/messages/count_tokens" || path == "/messages/count_tokens":
		w = withClientLogMeta(w, r)
		ar, release := h.authenticateForClaude(w, r)
		if ar == nil {
			return
		}
		defer release()
		h.handleCountTokens(w, ar)
	case path == "/v1/chat/completions" || path == "/chat/completions":
		w = withClientLogMeta(w, r)
		ar, release := h.authenticateForOpenAI(w, r)
		if ar == nil {
			return
		}
		defer release()
		h.handleOpenAIChat(w, ar)
	case path == "/v1/responses" || path == "/responses":
		w = withClientLogMeta(w, r)
		ar, release := h.authenticateForOpenAI(w, r)
		if ar == nil {
			return
		}
		defer release()
		h.handleOpenAIResponses(w, ar)
	case path == "/v1/usage":
		h.handleKeyUsage(w, r)
	case path == "/usage" || path == "/usage/" || path == "/docs" || strings.HasPrefix(path, "/docs/"):
		// Public pages (usage check, documentation): same SPA bundle, no admin path exposed.
		h.serveAdminPage(w, r)
	case path == "/v1/models" || path == "/models":
		h.handleModels(w, r)
	case path == "/api/event_logging/batch":
		// Claude Code 遥测端点 - 直接返回 200 OK
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"status":"ok"}`))

	// 管理端点
	case path == "/admin" || path == "/admin/":
		h.serveAdminPage(w, r)
	case strings.HasPrefix(path, "/admin/api/"):
		h.handleAdminAPI(w, r)
	case strings.HasPrefix(path, "/admin/"):
		h.serveStaticFile(w, r)

	// 健康检查
	case path == "/health" || path == "/":
		h.handleHealth(w, r)

	// 统计端点（需要 API Key 鉴权）
	case path == "/v1/stats":
		if !h.validateApiKey(r) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or missing API key"})
			return
		}
		h.handleStats(w, r)

	default:
		http.Error(w, "Not Found", 404)
	}
}

// handleKeyUsage is the public self-service usage check: a client presents its
// own API key (Bearer / X-Api-Key) and gets that key's quota and usage back.
// No admin credentials are involved; the key itself is the credential.
func (h *Handler) handleKeyUsage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	provided := extractProvidedKey(r)
	entry := config.FindApiKeyByValue(provided)
	if provided == "" || entry == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or missing API key"})
		return
	}
	pct := func(used, limit float64) float64 {
		if limit <= 0 {
			return 0
		}
		return used / limit
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":             entry.Name,
		"keyMasked":        config.MaskApiKey(entry.Key),
		"enabled":          entry.Enabled,
		"createdAt":        entry.CreatedAt,
		"lastUsedAt":       entry.LastUsedAt,
		"requestsCount":    entry.RequestsCount,
		"tokensUsed":       entry.TokensUsed,
		"tokenLimit":       entry.TokenLimit,
		"tokenPercent":     pct(float64(entry.TokensUsed), float64(entry.TokenLimit)),
		"creditsUsed":      entry.CreditsUsed,
		"creditLimit":      entry.CreditLimit,
		"creditPercent":    pct(entry.CreditsUsed, entry.CreditLimit),
		"rpmLimit":         entry.RPMLimit,
		"concurrencyLimit": entry.ConcurrencyLimit,
	})
}

// handleHealth 健康检查（不暴露统计数据）
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": config.Version,
		"uptime":  time.Now().Unix() - h.startTime,
	})
}

// handleStats 统计数据（需要 API Key 鉴权）
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "ok",
		"version":         config.Version,
		"accounts":        h.pool.Count(),
		"available":       h.pool.AvailableCount(),
		"totalRequests":   atomic.LoadInt64(&h.totalRequests),
		"successRequests": atomic.LoadInt64(&h.successRequests),
		"failedRequests":  atomic.LoadInt64(&h.failedRequests),
		"totalTokens":     atomic.LoadInt64(&h.totalTokens),
		"totalCredits":    h.getCredits(),
		"uptime":          time.Now().Unix() - h.startTime,
	})
}

// handleModels 模型列表
func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	// 尝试用缓存的真实模型列表
	h.modelsCacheMu.RLock()
	cached := h.cachedModels
	h.modelsCacheMu.RUnlock()
	if len(cached) == 0 {
		h.refreshModelsCache()
		h.modelsCacheMu.RLock()
		cached = h.cachedModels
		h.modelsCacheMu.RUnlock()
	}

	thinkingSuffix := config.GetThinkingConfig().Suffix

	models := buildAnthropicModelsResponse(cached, thinkingSuffix)
	if len(models) == 0 {
		models = fallbackAnthropicModels(thinkingSuffix)
	}

	// 添加别名模型
	models = append(models,
		buildModelInfo("auto", "kiro-proxy", true),
		buildModelInfo("gpt-4o", "kiro-proxy", true),
		buildModelInfo("gpt-4", "kiro-proxy", true),
	)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   models,
	})
	return
}

func buildAnthropicModelsResponse(cached []ModelInfo, thinkingSuffix string) []map[string]interface{} {
	if len(cached) == 0 {
		return nil
	}

	models := make([]map[string]interface{}, 0, len(cached)*2)
	if len(cached) > 0 {
		for _, m := range cached {
			supportsImage := modelSupportsImage(m.InputTypes)
			models = append(models, buildModelInfo(m.ModelId, "anthropic", supportsImage))
			// 自动生成 thinking 变体
			models = append(models, buildModelInfo(m.ModelId+thinkingSuffix, "anthropic", supportsImage))
		}
	}
	return models
}

func fallbackAnthropicModels(thinkingSuffix string) []map[string]interface{} {
	return []map[string]interface{}{
		buildModelInfo("claude-sonnet-4.6", "anthropic", true),
		buildModelInfo("claude-sonnet-4.6"+thinkingSuffix, "anthropic", true),
		buildModelInfo("claude-opus-4.6", "anthropic", true),
		buildModelInfo("claude-opus-4.6"+thinkingSuffix, "anthropic", true),
		buildModelInfo("claude-opus-4.7", "anthropic", true),
		buildModelInfo("claude-opus-4.7"+thinkingSuffix, "anthropic", true),
		buildModelInfo("claude-sonnet-4.5", "anthropic", true),
		buildModelInfo("claude-sonnet-4.5"+thinkingSuffix, "anthropic", true),
		buildModelInfo("claude-sonnet-4", "anthropic", true),
		buildModelInfo("claude-sonnet-4"+thinkingSuffix, "anthropic", true),
		buildModelInfo("claude-haiku-4.5", "anthropic", true),
		buildModelInfo("claude-haiku-4.5"+thinkingSuffix, "anthropic", true),
		buildModelInfo("claude-opus-4.5", "anthropic", true),
		buildModelInfo("claude-opus-4.5"+thinkingSuffix, "anthropic", true),
	}
}

func modelSupportsImage(inputTypes []string) bool {
	for _, t := range inputTypes {
		lt := strings.ToLower(t)
		if strings.Contains(lt, "image") || strings.Contains(lt, "vision") {
			return true
		}
	}
	return false
}

func buildModelInfo(id, ownedBy string, supportsImage bool) map[string]interface{} {
	modalities := []string{"text"}
	if supportsImage {
		modalities = append(modalities, "image")
	}
	modalitiesMap := map[string][]string{
		"input":  modalities,
		"output": []string{"text"},
	}

	return map[string]interface{}{
		"id":               id,
		"object":           "model",
		"owned_by":         ownedBy,
		"supports_image":   supportsImage,
		"input_modalities": modalities,
		"modalities":       modalitiesMap,
		"capabilities": map[string]bool{
			"vision":       supportsImage,
			"image":        supportsImage,
			"image_vision": supportsImage,
		},
		"info": map[string]interface{}{
			"meta": map[string]interface{}{
				"capabilities": map[string]bool{
					"vision":       supportsImage,
					"image_vision": supportsImage,
				},
			},
		},
	}
}

// refreshModelsCache 从 Kiro API 拉取模型列表并缓存
func (h *Handler) refreshModelsCache() {
	accounts := config.GetEnabledAccounts()
	if len(accounts) == 0 {
		return
	}

	aggregated := make([]ModelInfo, 0)
	for i := range accounts {
		account := &accounts[i]
		if err := h.ensureValidToken(account); err != nil {
			logger.Warnf("[ModelsCache] Skip %s token refresh failed: %v", account.Email, err)
			h.handleAccountFailure(account, err)
			continue
		}

		models, err := ListAvailableModels(account)
		if err != nil {
			logger.Warnf("[ModelsCache] Failed to refresh for %s: %v", account.Email, err)
			h.handleAccountFailure(account, err)
			continue
		}
		// 缓存每账号可用模型，用于路由时过滤
		modelIDs := make([]string, 0, len(models))
		for _, m := range models {
			modelIDs = append(modelIDs, m.ModelId)
		}
		h.pool.SetModelList(account.ID, modelIDs)
		aggregated = mergeUniqueModels(aggregated, models)
	}

	if len(aggregated) > 0 {
		h.modelsCacheMu.Lock()
		h.cachedModels = aggregated
		h.modelsCacheTime = time.Now().Unix()
		h.modelsCacheMu.Unlock()
		logger.Infof("[ModelsCache] Cached %d models", len(aggregated))
	}
}

// fetchAndCacheAccountModels 为单个账号拉取并写入模型缓存。
// 同时更新 pool 的路由缓存与全局聚合模型列表。
func (h *Handler) fetchAndCacheAccountModels(account *config.Account) error {
	if err := h.ensureValidToken(account); err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}
	models, err := ListAvailableModels(account)
	if err != nil {
		return err
	}
	modelIDs := make([]string, 0, len(models))
	for _, m := range models {
		modelIDs = append(modelIDs, m.ModelId)
	}
	h.pool.SetModelList(account.ID, modelIDs)

	// 合并到聚合缓存
	h.modelsCacheMu.Lock()
	h.cachedModels = mergeUniqueModels(h.cachedModels, models)
	h.modelsCacheTime = time.Now().Unix()
	h.modelsCacheMu.Unlock()

	logger.Infof("[ModelsCache] Refreshed %d models for account %s", len(models), account.Email)
	return nil
}

// apiRefreshAccountModels POST /admin/api/accounts/{id}/models/refresh
// 立即为指定账号拉取并更新模型路由缓存。
func (h *Handler) apiRefreshAccountModels(w http.ResponseWriter, r *http.Request, id string) {
	accounts := config.GetAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}
	// 从 pool 取运行时最新 token（与 refreshModelsCache 逻辑一致）
	if latest := h.pool.GetByID(id); latest != nil {
		account.AccessToken = latest.AccessToken
		account.RefreshToken = latest.RefreshToken
		account.ExpiresAt = latest.ExpiresAt
		account.ProfileArn = latest.ProfileArn
	}
	h.pool.ClearModelDenies(id) // operator asked for a fresh view: forget learned rejections
	if err := h.fetchAndCacheAccountModels(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   len(h.pool.GetModelList(id)),
	})
}

// apiRefreshAllAccountsModels POST /admin/api/accounts/models/refresh
// 直接复用 refreshModelsCache，为所有已启用账号刷新模型路由缓存。
func (h *Handler) apiRefreshAllAccountsModels(w http.ResponseWriter, r *http.Request) {
	h.refreshModelsCache()
	h.modelsCacheMu.RLock()
	cachedLen := len(h.cachedModels)
	h.modelsCacheMu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"refreshed": cachedLen,
		"failed":    0,
	})
}

func mergeUniqueModels(existing []ModelInfo, incoming []ModelInfo) []ModelInfo {
	if len(incoming) == 0 {
		return existing
	}

	indexByID := make(map[string]int, len(existing))
	merged := make([]ModelInfo, len(existing))
	copy(merged, existing)
	for i, model := range merged {
		indexByID[strings.ToLower(strings.TrimSpace(model.ModelId))] = i
	}

	for _, model := range incoming {
		key := strings.ToLower(strings.TrimSpace(model.ModelId))
		if key == "" {
			continue
		}
		if idx, ok := indexByID[key]; ok {
			merged[idx] = mergeModelInfo(merged[idx], model)
			continue
		}
		indexByID[key] = len(merged)
		merged = append(merged, model)
	}

	return merged
}

func mergeModelInfo(base ModelInfo, extra ModelInfo) ModelInfo {
	if base.ModelName == "" {
		base.ModelName = extra.ModelName
	}
	if base.Description == "" {
		base.Description = extra.Description
	}
	if base.RateMultiplier == 0 {
		base.RateMultiplier = extra.RateMultiplier
	}
	if base.TokenLimits == nil {
		base.TokenLimits = extra.TokenLimits
	}
	base.InputTypes = mergeStringLists(base.InputTypes, extra.InputTypes)
	return base
}

func mergeStringLists(base []string, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base)+len(extra))
	merged := make([]string, 0, len(base)+len(extra))
	for _, item := range base {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, item)
	}
	for _, item := range extra {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, item)
	}
	return merged
}

// handleCountTokens Token 计数（Claude Code 会调用）
func (h *Handler) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Failed to read request body")
		return
	}

	var req ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Invalid JSON")
		return
	}
	if msg := validateClaudeThinkingConfig(req.Thinking, req.MaxTokens); msg != "" {
		h.sendClaudeError(w, 400, "invalid_request_error", msg)
		return
	}

	thinkingCfg := config.GetThinkingConfig()
	actualModel, thinking := resolveClaudeThinkingMode(req.Model, req.Thinking, thinkingCfg.Suffix)
	req.Model = actualModel
	effectiveReq := cloneClaudeRequestForThinking(&req, thinking)

	estimatedTokens := estimateClaudeRequestInputTokens(effectiveReq)
	if estimatedTokens < 1 {
		estimatedTokens = 1
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]int{"input_tokens": estimatedTokens})
}

// handleClaudeMessages Claude API 处理
func (h *Handler) handleClaudeMessages(w http.ResponseWriter, r *http.Request) {
	h.handleClaudeMessagesInternal(w, r)
}

func (h *Handler) handleClaudeMessagesInternal(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	// 读取请求
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Failed to read request body")
		return
	}

	var req ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.sendClaudeError(w, 400, "invalid_request_error", "Invalid JSON: "+err.Error())
		return
	}
	if msg := validateClaudeRequestShape(&req); msg != "" {
		h.sendClaudeError(w, 400, "invalid_request_error", msg)
		return
	}

	// 解析模型和 thinking 模式
	thinkingCfg := config.GetThinkingConfig()
	actualModel, thinking := resolveClaudeThinkingMode(req.Model, req.Thinking, thinkingCfg.Suffix)
	req.Model = actualModel
	effectiveReq := cloneClaudeRequestForThinking(&req, thinking)
	thinkingResponseOpts := resolveClaudeThinkingResponseOptions(req.Thinking, thinkingCfg.ClaudeFormat)
	estimatedInputTokens := estimateClaudeRequestInputTokens(effectiveReq)
	cacheProfile := h.promptCache.BuildClaudeProfile(effectiveReq, estimatedInputTokens)

	// Stream or non-stream. Provider-specific conversion happens after account
	// selection inside the upstream routing layer.
	apiKeyID := apiKeyIDFromContext(r.Context())
	affinityKey := claudeAffinityKey(&req)
	if isAutoModel(req.Model) {
		req.Model = h.resolveAutoModel(w, "claude", req.Model, claudeRouteSignals(&req, estimatedInputTokens, thinking), &affinityKey, capClaudeChat)
		effectiveReq.Model = req.Model
		cacheProfile = h.promptCache.BuildClaudeProfile(effectiveReq, estimatedInputTokens)
	}
	if req.Stream {
		h.handleClaudeStream(w, &req, req.Model, thinking, thinkingResponseOpts, estimatedInputTokens, cacheProfile, apiKeyID, affinityKey)
	} else {
		h.handleClaudeNonStream(w, &req, req.Model, thinking, thinkingResponseOpts, estimatedInputTokens, cacheProfile, apiKeyID, affinityKey)
	}
}

// handleClaudeStream Claude 流式响应
func (h *Handler) handleClaudeStream(w http.ResponseWriter, req *ClaudeRequest, model string, thinking bool, thinkingOpts claudeThinkingResponseOptions, estimatedInputTokens int, cacheProfile *promptCacheProfile, apiKeyID string, affinityKey string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.sendClaudeError(w, 500, "api_error", "Streaming not supported")
		return
	}

	// 获取 thinking 输出格式配置
	thinkingFormat := thinkingOpts.Format

	reqStart := time.Now()
	perf := newPerfTracker(reqStart)
	msgID := "msg_" + uuid.New().String()
	startInputTokens := estimatedInputTokens
	excluded := make(map[string]bool)
	var lastErr error
	messageStarted := false
	var messageStartUsage promptCacheUsage

	ensureMessageStart := func() {
		if messageStarted {
			return
		}
		h.sendSSE(w, flusher, "message_start", map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":            msgID,
				"type":          "message",
				"role":          "assistant",
				"content":       []interface{}{},
				"model":         model,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage":         buildClaudeUsageMap(startInputTokens, 0, messageStartUsage, cacheProfile != nil),
			},
		})
		messageStarted = true
	}

	for attempt := 0; attempt < maxAccountRetryAttempts; attempt++ {
		account := h.pickAccount(affinityKey, model, excluded, capabilityFilter(capClaudeChat))
		if account == nil {
			break
		}
		if err := h.ensureValidToken(account); err != nil {
			lastErr = err
			excluded[account.ID] = true
			h.handleModelFailure(account, model, err)
			continue
		}
		cacheUsage := h.promptCache.Compute(account.ID, cacheProfile)
		messageStartUsage = cacheUsage

		var inputTokens, outputTokens int
		var credits float64
		var realInputTokens int
		var upstreamStop string
		var toolUses []ToolUse
		var nextContentIndex int
		var rawContentBuilder strings.Builder
		var rawThinkingBuilder strings.Builder
		activeBlockIndex := -1
		activeBlockType := ""

		closeActiveBlock := func() {
			if activeBlockIndex < 0 {
				return
			}
			h.sendSSE(w, flusher, "content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": activeBlockIndex,
			})
			activeBlockIndex = -1
			activeBlockType = ""
		}

		startContentBlock := func(blockType string) {
			if activeBlockType == blockType {
				return
			}
			ensureMessageStart()
			closeActiveBlock()

			idx := nextContentIndex
			nextContentIndex++

			if blockType == "thinking" {
				h.sendSSE(w, flusher, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": idx,
					"content_block": map[string]string{
						"type":     "thinking",
						"thinking": "",
					},
				})
			} else {
				h.sendSSE(w, flusher, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": idx,
					"content_block": map[string]string{
						"type": "text",
						"text": "",
					},
				})
			}

			activeBlockIndex = idx
			activeBlockType = blockType
		}

		var textBuffer string
		var inThinkingBlock bool
		var dropTagThinking bool
		var thinkingSource thinkingStreamSource
		var thinkingStarted bool
		var eventThinkingOpen bool

		sendText := func(text string, thinkingState int) {
			if thinkingState == 0 {
				if text == "" {
					return
				}
				startContentBlock("text")
				h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": activeBlockIndex,
					"delta": map[string]string{"type": "text_delta", "text": text},
				})
				return
			}

			if !thinking {
				return
			}

			switch thinkingFormat {
			case "think":
				var outputText string
				switch thinkingState {
				case 1:
					outputText = "<think>" + text
				case 2:
					outputText = text
				case 3:
					outputText = text + "</think>"
				}
				if outputText == "" {
					return
				}
				startContentBlock("text")
				h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": activeBlockIndex,
					"delta": map[string]string{"type": "text_delta", "text": outputText},
				})
			case "reasoning_content":
				if text == "" {
					return
				}
				startContentBlock("text")
				h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": activeBlockIndex,
					"delta": map[string]string{"type": "text_delta", "text": text},
				})
			default:
				if thinkingOpts.OmitDisplay {
					if thinkingState == 1 {
						startContentBlock("thinking")
						return
					}
					if thinkingState == 3 {
						if activeBlockType != "thinking" {
							startContentBlock("thinking")
						}
						closeActiveBlock()
					}
					return
				}
				if thinkingState == 3 && text == "" {
					if activeBlockType == "thinking" {
						closeActiveBlock()
					}
					return
				}
				if text != "" {
					startContentBlock("thinking")
					h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
						"type":  "content_block_delta",
						"index": activeBlockIndex,
						"delta": map[string]string{"type": "thinking_delta", "thinking": text},
					})
				}
				if thinkingState == 3 && activeBlockType == "thinking" {
					closeActiveBlock()
				}
			}
		}

		processClaudeText := func(text string, isThinking bool, forceFlush bool) {
			if isThinking && !thinking {
				return
			}

			if isThinking {
				if !allowReasoningSource(&thinkingSource) {
					return
				}
				if !thinkingStarted {
					sendText(text, 1)
					thinkingStarted = true
					eventThinkingOpen = true
				} else {
					sendText(text, 2)
				}
				return
			}

			if eventThinkingOpen {
				sendText("", 3)
				eventThinkingOpen = false
				thinkingStarted = false
			}

			textBuffer += text

			for {
				if !inThinkingBlock {
					thinkingStart := strings.Index(textBuffer, "<thinking>")
					if thinkingStart != -1 {
						if thinkingStart > 0 {
							sendText(textBuffer[:thinkingStart], 0)
						}
						textBuffer = textBuffer[thinkingStart+10:]
						inThinkingBlock = true
						dropTagThinking = !allowTagSource(&thinkingSource)
						thinkingStarted = false
					} else if forceFlush || len([]rune(textBuffer)) > 50 {
						runes := []rune(textBuffer)
						safeLen := len(runes)
						if !forceFlush {
							safeLen = max(0, len(runes)-15)
						}
						if safeLen > 0 {
							sendText(string(runes[:safeLen]), 0)
							textBuffer = string(runes[safeLen:])
						}
						break
					} else {
						break
					}
				} else {
					thinkingEnd := strings.Index(textBuffer, "</thinking>")
					if thinkingEnd != -1 {
						content := textBuffer[:thinkingEnd]
						if !dropTagThinking {
							if !thinkingStarted {
								sendText(content, 1)
								sendText("", 3)
							} else {
								sendText(content, 3)
							}
						}
						textBuffer = textBuffer[thinkingEnd+11:]
						inThinkingBlock = false
						dropTagThinking = false
						thinkingStarted = false
					} else if forceFlush {
						if textBuffer != "" {
							if !dropTagThinking {
								if !thinkingStarted {
									sendText(textBuffer, 1)
									sendText("", 3)
								} else {
									sendText(textBuffer, 3)
								}
							}
							textBuffer = ""
						}
						inThinkingBlock = false
						dropTagThinking = false
						thinkingStarted = false
						break
					} else {
						runes := []rune(textBuffer)
						if len(runes) > 20 {
							safeLen := len(runes) - 15
							if safeLen > 0 {
								if !dropTagThinking {
									if !thinkingStarted {
										sendText(string(runes[:safeLen]), 1)
										thinkingStarted = true
									} else {
										sendText(string(runes[:safeLen]), 2)
									}
								}
								textBuffer = string(runes[safeLen:])
							}
						}
						break
					}
				}
			}
		}

		callback := &StreamCallback{
			OnText: func(text string, isThinking bool) {
				if text == "" {
					return
				}
				perf.markFirstByte()
				perf.addTokens(text)
				if isThinking {
					rawThinkingBuilder.WriteString(text)
				} else {
					rawContentBuilder.WriteString(text)
				}
				processClaudeText(text, isThinking, false)
			},
			OnToolUse: func(tu ToolUse) {
				perf.markFirstByte()
				processClaudeText("", false, true)
				rawContentBuilder.WriteString(tu.Name)
				if b, err := json.Marshal(tu.Input); err == nil {
					rawContentBuilder.Write(b)
				}

				toolUses = append(toolUses, tu)
				ensureMessageStart()
				closeActiveBlock()

				idx := nextContentIndex
				nextContentIndex++

				h.sendSSE(w, flusher, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": idx,
					"content_block": map[string]interface{}{
						"type":  "tool_use",
						"id":    tu.ToolUseID,
						"name":  tu.Name,
						"input": map[string]interface{}{},
					},
				})

				inputJSON, _ := json.Marshal(tu.Input)
				h.sendSSE(w, flusher, "content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": idx,
					"delta": map[string]interface{}{
						"type":         "input_json_delta",
						"partial_json": string(inputJSON),
					},
				})

				h.sendSSE(w, flusher, "content_block_stop", map[string]interface{}{
					"type":  "content_block_stop",
					"index": idx,
				})
			},
			OnComplete: func(inTok, outTok int) {
				inputTokens = inTok
				outputTokens = outTok
			},
			OnCredits: func(c float64) {
				credits = c
			},
			OnContextUsage: func(pct float64) {
				realInputTokens = int(pct * float64(getContextWindowSize(model)) / 100.0)
			},
			OnStopReason: func(reason string) { upstreamStop = reason },
		}

		err := callUpstreamFromClaude(account, req, thinking, callback)
		if err != nil {
			lastErr = err
			excluded[account.ID] = true
			h.handleModelFailure(account, model, err)
			if !messageStarted {
				continue
			}
			h.recordFailureWithDetails(w, "claude", model, account.ID, err)
			h.sendSSE(w, flusher, "error", map[string]interface{}{
				"type":  "error",
				"error": map[string]string{"type": "api_error", "message": err.Error()},
			})
			return
		}

		processClaudeText("", false, true)
		if eventThinkingOpen {
			sendText("", 3)
		}
		closeActiveBlock()

		if realInputTokens > 0 {
			inputTokens = realInputTokens
		} else if inputTokens <= 0 {
			inputTokens = estimatedInputTokens
		}
		outputContent, extractedReasoning := extractThinkingFromContent(rawContentBuilder.String())
		thinkingOutput := rawThinkingBuilder.String()
		if thinking && thinkingOutput == "" && extractedReasoning != "" {
			thinkingOutput = extractedReasoning
		}
		if !thinking {
			thinkingOutput = ""
		}
		outputTokens = estimateClaudeOutputTokens(outputContent, thinkingOutput, toolUses)

		h.recordSuccessForApiKey(apiKeyID, inputTokens, outputTokens, credits)
		h.pool.RecordSuccess(account.ID)
		h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)
		h.promptCache.Update(account.ID, cacheProfile)
		h.recordSuccessLog(w, "claude", model, account.ID, inputTokens+outputTokens, credits, time.Since(reqStart).Milliseconds(), perf.finalise())
		stopReason := "end_turn"
		if len(toolUses) > 0 {
			stopReason = "tool_use"
		} else if upstreamStop == "max_tokens" {
			stopReason = "max_tokens"
		}

		ensureMessageStart()
		h.sendSSE(w, flusher, "message_delta", map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": stopReason,
			},
			"usage": buildClaudeUsageMap(inputTokens, outputTokens, cacheUsage, cacheProfile != nil),
		})

		h.sendSSE(w, flusher, "message_stop", map[string]interface{}{
			"type": "message_stop",
		})
		return
	}

	if lastErr == nil {
		h.sendClaudeError(w, 503, "api_error", "No available accounts")
		return
	}

	h.recordFailureWithDetails(w, "claude", model, "", lastErr)
	h.sendClaudeError(w, 500, "api_error", lastErr.Error())
}

func (h *Handler) sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
	flusher.Flush()
}

// backgroundStatsSaver 后台定时保存统计数据
func (h *Handler) backgroundStatsSaver() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.saveStats()
		case <-h.stopStatsSaver:
			h.saveStats() // 退出前保存一次
			return
		}
	}
}

// saveStats 保存统计到配置文件
func (h *Handler) saveStats() {
	config.UpdateStats(
		int(atomic.LoadInt64(&h.totalRequests)),
		int(atomic.LoadInt64(&h.successRequests)),
		int(atomic.LoadInt64(&h.failedRequests)),
		int(atomic.LoadInt64(&h.totalTokens)),
		h.getCredits(),
	)
}

// getCredits 线程安全获取 credits
func (h *Handler) getCredits() float64 {
	h.creditsMu.RLock()
	defer h.creditsMu.RUnlock()
	return h.totalCredits
}

// addCredits 线程安全增加 credits
func (h *Handler) addCredits(credits float64) {
	h.creditsMu.Lock()
	h.totalCredits += credits
	h.creditsMu.Unlock()
}

// dashboard "Performance" column needs. ttftMs is the gap between request
// start and the first upstream token; tps is outputTokens / seconds-after-TTFT.
// A zero value is a valid signal that the request was non-streaming (or no
// token arrived), and the log row simply renders the cells blank.
type requestPerf struct {
	ttftMs int64
	tps    float64
}

// perfTracker accumulates streaming timing as the callback fires. It snapshots
// the request start (which every handler already keeps) so first-token latency
// can be derived without the caller threading another timestamp around.
// finalise() returns the requestPerf to hand to recordSuccessLog.
type perfTracker struct {
	start      time.Time
	firstAt    time.Time // set on first non-empty OnText / OnToolUse / OnComplete
	tokenCount int       // output tokens seen at finalise()
	mu         sync.Mutex
}

// newPerfTracker constructs a tracker anchored to the handler's request start.
func newPerfTracker(start time.Time) *perfTracker {
	return &perfTracker{start: start}
}

// markFirstByte records the first time the upstream produced anything
// (token, tool_use, or the usage finalisation). Safe under concurrent calls
// from OnText / OnToolUse: only the earliest timestamp wins.
func (p *perfTracker) markFirstByte() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.firstAt.IsZero() {
		p.firstAt = time.Now()
	}
	p.mu.Unlock()
}

// addTokens bumps the running output-token estimate as the callback delivers
// text. We count by rune length / 4 (heuristic) rather than waiting for the
// final usage, so the TPS denominator reflects what the user actually saw
// arrive, not the post-hoc accounting.
func (p *perfTracker) addTokens(text string) {
	if p == nil || text == "" {
		return
	}
	p.mu.Lock()
	p.tokenCount += (utf8.RuneCountInString(text) + 3) / 4
	p.mu.Unlock()
}

// setFinalTokens overrides the estimate with the authoritative token count
// from the upstream usage block (OpenAI's prompt_tokens/completion_tokens).
func (p *perfTracker) setFinalTokens(n int) {
	if p == nil || n <= 0 {
		return
	}
	p.mu.Lock()
	p.tokenCount = n
	p.mu.Unlock()
}

// finalise produces the requestPerf to ship to the log row. Uses total wall
// time (request start → completion) for the TPS denominator; falls back to
// a 1 ms floor so a sub-millisecond request doesn't divide by zero.
func (p *perfTracker) finalise() requestPerf {
	if p == nil {
		return requestPerf{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	ttft := int64(0)
	if !p.firstAt.IsZero() && p.firstAt.After(p.start) {
		ttft = p.firstAt.Sub(p.start).Milliseconds()
	}
	total := now.Sub(p.start).Seconds()
	if total < 0.001 {
		total = 0.001
	}
	tps := float64(p.tokenCount) / total
	return requestPerf{ttftMs: ttft, tps: math.Round(tps*100) / 100}
}

// newLogEntry is the central constructor for RequestLog. It stamps time, the
// client metadata snapshot (taken via currentLogContext), and folds the perf
// numbers into the typed fields. Keeping this in one place avoids the dozen
// inline RequestLog{} literals scattered across handlers from drifting apart.
func (h *Handler) newLogEntry(w http.ResponseWriter, endpoint, model, accountID, status string, tokens, outputTokens int, credits float64, durationMs, ttftMs int64, tps float64) RequestLog {
	ua, ip := clientLogMeta(w)
	return RequestLog{
		Time:         time.Now().Unix(),
		Endpoint:     endpoint,
		Model:        model,
		AccountID:    accountID,
		Status:       status,
		Tokens:       tokens,
		OutputTokens: outputTokens,
		Credits:      credits,
		Duration:     durationMs,
		TTFTMs:       ttftMs,
		TPS:          tps,
		UserAgent:    ua,
		ClientIP:     ip,
	}
}

// appendRequestLog stores one entry in the in-memory ring buffer (newest at the
// tail, ring-shift eviction when full) and non-blockingly enqueues the same
// entry onto the persistence channel so the flusher goroutine can batch-write
// it to disk.
func (h *Handler) appendRequestLog(entry RequestLog) {
	h.requestLogsMu.Lock()
	if h.requestLogs == nil {
		h.requestLogs = make([]RequestLog, 0, requestLogsMaxSize)
	}
	if len(h.requestLogs) >= requestLogsMaxSize {
		h.requestLogs = h.requestLogs[1:]
	}
	h.requestLogs = append(h.requestLogs, entry)
	h.requestLogsMu.Unlock()

	// 非阻塞投递到持久化队列。队列满 = 上一批次还没刷盘，降级为丢弃本次
	// 持久化条目（内存环照常保留，查询不丢），避免反向阻塞请求路径。
	if h.pendingLogs != nil {
		persisted := config.PersistedRequestLog{
			Time: entry.Time, Endpoint: entry.Endpoint, Model: entry.Model,
			AccountID: entry.AccountID, Status: entry.Status,
			Error: entry.Error, ErrorType: entry.ErrorType,
			Tokens: entry.Tokens, OutputTokens: entry.OutputTokens,
			Credits: entry.Credits, Duration: entry.Duration,
			TTFTMs: entry.TTFTMs, TPS: entry.TPS,
			UserAgent: entry.UserAgent, ClientIP: entry.ClientIP,
		}
		select {
		case h.pendingLogs <- persisted:
		default:
			// 后台正在批量写入；下一轮再补。这里不再 spin / 阻塞。
		}
	}
}

// recordFailureWithDetails records a failure and stores it in the request logs.
// log row renders empty TTFT/TPS cells.
func (h *Handler) recordFailureWithDetails(w http.ResponseWriter, endpoint, model, accountID string, err error) {
	atomic.AddInt64(&h.totalRequests, 1)
	atomic.AddInt64(&h.failedRequests, 1)

	if err == nil {
		return
	}

	errMsg := err.Error()
	errType := classifyError(errMsg)

	entry := h.newLogEntry(w, endpoint, model, accountID, "error", 0, 0, 0, 0, 0, 0)
	entry.Error = errMsg
	entry.ErrorType = errType

	h.appendRequestLog(entry)
	h.autoRouter.Record(accountID, model, false, 0)
	h.metrics.Record(endpoint, model, accountID, false, 0, 0, 0)
}

// recordSuccessForApiKey credits the per-key rate limiter and quota counters.
// Kept separate from recordSuccessLog so log-row perf timing (TTFT/TPS) can
// evolve without touching the limiter. The current keyLimiter has no usage
// recorder (Acquire/Release only); this is the seam future per-key quota
// dashboards can plug into without touching call sites.
func (h *Handler) recordSuccessForApiKey(apiKeyID string, inputTokens, outputTokens int, credits float64) {
	if apiKeyID == "" {
		return
	}
	// Sum prompt + completion tokens so the quota counters match the per-request
	// bill. RecordApiKeyUsage is a no-op for non-positive token/credit deltas, so
	// upstream callers that pass zeros (e.g. responses streaming paths before
	// the model reports usage) don't pollute the counters.
	totalTokens := int64(inputTokens) + int64(outputTokens)
	if err := config.RecordApiKeyUsage(apiKeyID, totalTokens, credits); err != nil {
		logger.Warnf("[apikey] record usage for %s failed: %v", apiKeyID, err)
	}
}

// recordSuccessLog records a successful request in the request logs. The
// perf struct carries streaming-only timing: TTFT (first-byte to first token)
// and the TPS denominator (outputTokens / max(secondsAfterFirstByte, 1ms)).
// Pass a zero `perf` for non-streaming endpoints to log no TTFT/TPS.
func (h *Handler) recordSuccessLog(w http.ResponseWriter, endpoint, model, accountID string, tokens int, credits float64, durationMs int64, perf requestPerf) {
	atomic.AddInt64(&h.totalRequests, 1)
	atomic.AddInt64(&h.successRequests, 1)
	atomic.AddInt64(&h.totalTokens, int64(tokens))
	h.addCredits(credits)

	entry := h.newLogEntry(w, endpoint, model, accountID, "success", tokens, 0, credits, durationMs, perf.ttftMs, perf.tps)

	h.appendRequestLog(entry)
	h.autoRouter.Record(accountID, model, true, durationMs)
	h.metrics.Record(endpoint, model, accountID, true, tokens, credits, durationMs)
}

// 开销过大；太长：崩溃丢失窗口过大。5s 与现有 metrics 刷新节奏一致。
const requestLogFlushInterval = 5 * time.Second

// requestLogBatchSize 触发立即 flush 的条目阈值，避免突发后长时间不刷盘。
const requestLogBatchSize = 64

// requestLogPendingCapacity 是 pendingLogs 通道的缓冲大小。环形 500 条 +
// 突发，1024 给后台 5s 周期足够吸纳一次小高峰；满了就走非阻塞丢弃路径。
const requestLogPendingCapacity = 1024

// runRequestLogFlusher 从 pendingLogs 收集条目并按周期批量持久化。
// store 为 nil 时（极少数后端未实现）直接退出，保持原内存-only 行为。
func (h *Handler) runRequestLogFlusher(store config.RequestLogStore) {
	defer close(h.logSaverDone)
	ticker := time.NewTicker(requestLogFlushInterval)
	defer ticker.Stop()

	batch := make([]config.PersistedRequestLog, 0, requestLogBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := store.Append(batch); err != nil {
			logger.Warnf("request log flush failed: %v (lost %d entries)", err, len(batch))
		}
		batch = batch[:0]
	}

	for {
		select {
		case e, ok := <-h.pendingLogs:
			if !ok {
				flush()
				return
			}
			batch = append(batch, e)
			if len(batch) >= requestLogBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-h.stopLogSaver:
			// 退出前先把通道里所有剩余条目拉空再 flush，保证 Shutdown 同步落盘。
			for {
				select {
				case e := <-h.pendingLogs:
					batch = append(batch, e)
				default:
					flush()
					return
				}
			}
		}
	}
}

// classifyError categorizes an error message into a type for display.
func classifyError(msg string) string {
	switch {
	case isQuotaErrorMessage(msg):
		return "quota"
	case isOverageErrorMessage(msg):
		return "overage"
	case isSuspensionErrorMessage(msg):
		return "suspended"
	case isAuthErrorMessage(msg):
		return "auth"
	case isProfileUnavailableErrorMessage(msg):
		return "profile"
	default:
		return "unknown"
	}
}

// getRequestLogs returns a copy of request logs (newest first).
func (h *Handler) getRequestLogs() []RequestLog {
	h.requestLogsMu.RLock()
	defer h.requestLogsMu.RUnlock()
	if len(h.requestLogs) == 0 {
		return []RequestLog{}
	}
	result := make([]RequestLog, len(h.requestLogs))
	for i, e := range h.requestLogs {
		result[len(h.requestLogs)-1-i] = e
	}
	return result
}

// handleClaudeNonStream Claude 非流式响应
func (h *Handler) handleClaudeNonStream(w http.ResponseWriter, req *ClaudeRequest, model string, thinking bool, thinkingOpts claudeThinkingResponseOptions, estimatedInputTokens int, cacheProfile *promptCacheProfile, apiKeyID string, affinityKey string) {
	excluded := make(map[string]bool)
	var lastErr error
	reqStart := time.Now()
	perf := newPerfTracker(reqStart)

	for attempt := 0; attempt < maxAccountRetryAttempts; attempt++ {
		account := h.pickAccount(affinityKey, model, excluded, capabilityFilter(capClaudeChat))
		if account == nil {
			break
		}
		if err := h.ensureValidToken(account); err != nil {
			lastErr = err
			excluded[account.ID] = true
			h.handleModelFailure(account, model, err)
			continue
		}
		cacheUsage := h.promptCache.Compute(account.ID, cacheProfile)

		var content string
		var thinkingContent string
		var toolUses []ToolUse
		var inputTokens, outputTokens int
		var credits float64
		var realInputTokens int

		callback := &StreamCallback{
			OnText: func(text string, isThinking bool) {
				perf.markFirstByte()
				perf.addTokens(text)
				if isThinking {
					thinkingContent += text
				} else {
					content += text
				}
			},
			OnToolUse: func(tu ToolUse) {
				perf.markFirstByte()
				toolUses = append(toolUses, tu)
			},
			OnComplete: func(inTok, outTok int) {
				perf.setFinalTokens(outTok)
				inputTokens = inTok
				outputTokens = outTok
			},
			OnCredits: func(c float64) {
				credits = c
			},
			OnContextUsage: func(pct float64) {
				realInputTokens = int(pct * float64(getContextWindowSize(model)) / 100.0)
			},
		}

		err := callUpstreamFromClaude(account, req, thinking, callback)
		if err != nil {
			lastErr = err
			excluded[account.ID] = true
			h.handleModelFailure(account, model, err)
			continue
		}

		thinkingFormat := thinkingOpts.Format
		finalContent, extractedReasoning := extractThinkingFromContent(content)
		rawThinkingContent := thinkingContent
		if thinking && rawThinkingContent == "" && extractedReasoning != "" {
			rawThinkingContent = extractedReasoning
		}
		if !thinking {
			rawThinkingContent = ""
		}
		if realInputTokens > 0 {
			inputTokens = realInputTokens
		} else if inputTokens <= 0 {
			inputTokens = estimatedInputTokens
		}
		outputTokens = estimateClaudeOutputTokens(finalContent, rawThinkingContent, toolUses)

		h.recordSuccessForApiKey(apiKeyID, inputTokens, outputTokens, credits)
		h.pool.RecordSuccess(account.ID)
		h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)
		h.recordSuccessLog(w, "claude", model, account.ID, inputTokens+outputTokens, credits, time.Since(reqStart).Milliseconds(), perf.finalise())

		responseThinkingContent := rawThinkingContent
		includeEmptyThinkingBlock := thinking && thinkingOpts.OmitDisplay && rawThinkingContent != ""
		if includeEmptyThinkingBlock {
			responseThinkingContent = ""
		}

		if thinking && responseThinkingContent != "" {
			switch thinkingFormat {
			case "think":
				finalContent = "<think>" + responseThinkingContent + "</think>" + finalContent
				responseThinkingContent = ""
			case "reasoning_content":
				finalContent = responseThinkingContent + finalContent
				responseThinkingContent = ""
			default:
			}
		}

		resp := KiroToClaudeResponse(finalContent, responseThinkingContent, includeEmptyThinkingBlock, toolUses, inputTokens, outputTokens, model)
		resp.Usage.InputTokens = billedClaudeInputTokens(inputTokens, cacheUsage)
		resp.Usage.CacheCreationInputTokens = cacheUsage.CacheCreationInputTokens
		resp.Usage.CacheReadInputTokens = cacheUsage.CacheReadInputTokens
		if cacheProfile != nil {
			resp.Usage.CacheCreation = &ClaudeCacheCreationUsage{
				Ephemeral5mInputTokens: cacheUsage.CacheCreation5mInputTokens,
				Ephemeral1hInputTokens: cacheUsage.CacheCreation1hInputTokens,
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)
		return
	}

	if lastErr == nil {
		h.sendClaudeError(w, 503, "api_error", "No available accounts")
		return
	}

	h.recordFailureWithDetails(w, "claude", model, "", lastErr)
	h.sendClaudeError(w, 500, "api_error", lastErr.Error())
}

func (h *Handler) sendClaudeError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    errType,
			"message": message,
		},
	})
}

// handleOpenAIChat OpenAI API 处理
func (h *Handler) handleOpenAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendOpenAIError(w, 400, "invalid_request_error", "Failed to read request body")
		return
	}

	var req OpenAIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.sendOpenAIError(w, 400, "invalid_request_error", "Invalid JSON")
		return
	}
	if msg := validateOpenAIRequestShape(&req); msg != "" {
		h.sendOpenAIError(w, 400, "invalid_request_error", msg)
		return
	}

	// 解析模型和 thinking 模式
	thinkingCfg := config.GetThinkingConfig()
	actualModel, thinking := ParseModelAndThinking(req.Model, thinkingCfg.Suffix)
	req.Model = actualModel
	estimatedInputTokens := estimateOpenAIRequestInputTokens(&req)

	apiKeyID := apiKeyIDFromContext(r.Context())
	affinityKey := openAIAffinityKey(&req)
	if isAutoModel(req.Model) {
		req.Model = h.resolveAutoModel(w, "openai", req.Model, openAIRouteSignals(&req, estimatedInputTokens, thinking), &affinityKey, capOpenAIChat)
	}
	if req.Stream {
		h.handleOpenAIStream(w, &req, req.Model, thinking, estimatedInputTokens, apiKeyID, affinityKey)
	} else {
		h.handleOpenAINonStream(w, &req, req.Model, thinking, estimatedInputTokens, apiKeyID, affinityKey)
	}
}

// handleOpenAIStream OpenAI 流式响应
func (h *Handler) handleOpenAIStream(w http.ResponseWriter, req *OpenAIRequest, model string, thinking bool, estimatedInputTokens int, apiKeyID string, affinityKey string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.sendOpenAIError(w, 500, "server_error", "Streaming not supported")
		return
	}

	// 获取 thinking 输出格式配置
	thinkingFormat := config.GetThinkingConfig().OpenAIFormat

	chatID := "chatcmpl-" + uuid.New().String()
	excluded := make(map[string]bool)
	var lastErr error
	reqStart := time.Now()
	perf := newPerfTracker(reqStart)
	for attempt := 0; attempt < maxAccountRetryAttempts; attempt++ {
		account := h.pickAccount(affinityKey, model, excluded, capabilityFilter(capOpenAIChat))
		if account == nil {
			break
		}
		if err := h.ensureValidToken(account); err != nil {
			lastErr = err
			excluded[account.ID] = true
			h.handleModelFailure(account, model, err)
			continue
		}

		var toolCalls []ToolCall
		var toolCallIndex int
		var inputTokens, outputTokens int
		var credits float64
		var realInputTokens int
		var rawContentBuilder strings.Builder
		var rawReasoningBuilder strings.Builder
		var textBuffer string
		var inThinkingBlock bool
		var dropTagThinking bool
		var thinkingSource thinkingStreamSource
		var thinkingStarted bool
		var eventThinkingOpen bool
		responseStarted := false

		sendChunk := func(content string, thinkingState int) {
			if content == "" && thinkingState == 2 {
				return
			}

			var chunk map[string]interface{}

			if thinkingState > 0 {
				if !thinking {
					return
				}
				switch thinkingFormat {
				case "thinking":
					var text string
					switch thinkingState {
					case 1:
						text = "<thinking>" + content
					case 2:
						text = content
					case 3:
						text = content + "</thinking>"
					}
					if text == "" {
						return
					}
					chunk = map[string]interface{}{
						"id":      chatID,
						"object":  "chat.completion.chunk",
						"created": time.Now().Unix(),
						"model":   model,
						"choices": []map[string]interface{}{{
							"index":         0,
							"delta":         map[string]string{"content": text},
							"finish_reason": nil,
						}},
					}
				case "think":
					var text string
					switch thinkingState {
					case 1:
						text = "<think>" + content
					case 2:
						text = content
					case 3:
						text = content + "</think>"
					}
					if text == "" {
						return
					}
					chunk = map[string]interface{}{
						"id":      chatID,
						"object":  "chat.completion.chunk",
						"created": time.Now().Unix(),
						"model":   model,
						"choices": []map[string]interface{}{{
							"index":         0,
							"delta":         map[string]string{"content": text},
							"finish_reason": nil,
						}},
					}
				default:
					if content == "" {
						return
					}
					chunk = map[string]interface{}{
						"id":      chatID,
						"object":  "chat.completion.chunk",
						"created": time.Now().Unix(),
						"model":   model,
						"choices": []map[string]interface{}{{
							"index":         0,
							"delta":         map[string]string{"reasoning_content": content},
							"finish_reason": nil,
						}},
					}
				}
			} else {
				if content == "" {
					return
				}
				chunk = map[string]interface{}{
					"id":      chatID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   model,
					"choices": []map[string]interface{}{{
						"index":         0,
						"delta":         map[string]string{"content": content},
						"finish_reason": nil,
					}},
				}
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
			responseStarted = true
		}

		processText := func(text string, isThinking bool, forceFlush bool) {
			if isThinking && !thinking {
				return
			}

			if isThinking {
				if !allowReasoningSource(&thinkingSource) {
					return
				}
				if !thinkingStarted {
					sendChunk(text, 1)
					thinkingStarted = true
					eventThinkingOpen = true
				} else {
					sendChunk(text, 2)
				}
				return
			}

			if eventThinkingOpen {
				sendChunk("", 3)
				eventThinkingOpen = false
				thinkingStarted = false
			}

			textBuffer += text

			for {
				if !inThinkingBlock {
					thinkingStart := strings.Index(textBuffer, "<thinking>")
					if thinkingStart != -1 {
						if thinkingStart > 0 {
							sendChunk(textBuffer[:thinkingStart], 0)
						}
						textBuffer = textBuffer[thinkingStart+10:]
						inThinkingBlock = true
						dropTagThinking = !allowTagSource(&thinkingSource)
						thinkingStarted = false
					} else if forceFlush || len([]rune(textBuffer)) > 50 {
						runes := []rune(textBuffer)
						safeLen := len(runes)
						if !forceFlush {
							safeLen = max(0, len(runes)-15)
						}
						if safeLen > 0 {
							sendChunk(string(runes[:safeLen]), 0)
							textBuffer = string(runes[safeLen:])
						}
						break
					} else {
						break
					}
				} else {
					thinkingEnd := strings.Index(textBuffer, "</thinking>")
					if thinkingEnd != -1 {
						content := textBuffer[:thinkingEnd]
						if !dropTagThinking {
							if !thinkingStarted {
								sendChunk(content, 1)
								sendChunk("", 3)
							} else {
								sendChunk(content, 3)
							}
						}
						textBuffer = textBuffer[thinkingEnd+11:]
						inThinkingBlock = false
						dropTagThinking = false
						thinkingStarted = false
					} else if forceFlush {
						if textBuffer != "" {
							if !dropTagThinking {
								if !thinkingStarted {
									sendChunk(textBuffer, 1)
									sendChunk("", 3)
								} else {
									sendChunk(textBuffer, 3)
								}
							}
							textBuffer = ""
						}
						inThinkingBlock = false
						dropTagThinking = false
						thinkingStarted = false
						break
					} else {
						runes := []rune(textBuffer)
						if len(runes) > 20 {
							safeLen := len(runes) - 15
							if safeLen > 0 {
								if !dropTagThinking {
									if !thinkingStarted {
										sendChunk(string(runes[:safeLen]), 1)
										thinkingStarted = true
									} else {
										sendChunk(string(runes[:safeLen]), 2)
									}
								}
								textBuffer = string(runes[safeLen:])
							}
						}
						break
					}
				}
			}
		}

		callback := &StreamCallback{
			OnText: func(text string, isThinking bool) {
				if text == "" {
					return
				}
				perf.markFirstByte()
				perf.addTokens(text)
				if isThinking {
					rawReasoningBuilder.WriteString(text)
				} else {
					rawContentBuilder.WriteString(text)
				}
				processText(text, isThinking, false)
			},
			OnToolUse: func(tu ToolUse) {
				perf.markFirstByte()
				processText("", false, true)

				args, _ := json.Marshal(tu.Input)
				rawContentBuilder.WriteString(tu.Name)
				rawContentBuilder.Write(args)
				tc := ToolCall{ID: tu.ToolUseID, Type: "function"}
				tc.Function.Name = tu.Name
				tc.Function.Arguments = string(args)
				toolCalls = append(toolCalls, tc)

				chunk := map[string]interface{}{
					"id":      chatID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   model,
					"choices": []map[string]interface{}{{
						"index": 0,
						"delta": map[string]interface{}{
							"tool_calls": []map[string]interface{}{{
								"index": toolCallIndex,
								"id":    tu.ToolUseID,
								"type":  "function",
								"function": map[string]string{
									"name":      tu.Name,
									"arguments": string(args),
								},
							}},
						},
						"finish_reason": nil,
					}},
				}
				toolCallIndex++
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: %s\n\n", string(data))
				flusher.Flush()
				responseStarted = true
			},
			OnComplete: func(inTok, outTok int) {
				perf.setFinalTokens(outTok)
				inputTokens = inTok
				outputTokens = outTok
			},
			OnCredits: func(c float64) { credits = c },
			OnContextUsage: func(pct float64) {
				realInputTokens = int(pct * float64(getContextWindowSize(model)) / 100.0)
			},
		}

		err := callUpstreamFromOpenAI(account, req, thinking, callback)
		if err != nil {
			lastErr = err
			excluded[account.ID] = true
			h.handleModelFailure(account, model, err)
			if !responseStarted {
				continue
			}
			h.recordFailureWithDetails(w, "openai", model, account.ID, err)
			return
		}

		processText("", false, true)
		if eventThinkingOpen {
			sendChunk("", 3)
		}

		if realInputTokens > 0 {
			inputTokens = realInputTokens
		} else if inputTokens <= 0 {
			inputTokens = estimatedInputTokens
		}
		outputContent, extractedReasoning := extractThinkingFromContent(rawContentBuilder.String())
		reasoningOutput := rawReasoningBuilder.String()
		if thinking && reasoningOutput == "" && extractedReasoning != "" {
			reasoningOutput = extractedReasoning
		}
		if !thinking {
			reasoningOutput = ""
		}
		outputTokens = estimateApproxTokens(outputContent) + estimateApproxTokens(reasoningOutput)
		for _, tc := range toolCalls {
			outputTokens += estimateApproxTokens(tc.Function.Name)
			outputTokens += estimateApproxTokens(tc.Function.Arguments)
		}

		h.recordSuccessForApiKey(apiKeyID, inputTokens, outputTokens, credits)
		h.pool.RecordSuccess(account.ID)
		h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)
		h.recordSuccessLog(w, "openai", model, account.ID, inputTokens+outputTokens, credits, time.Since(reqStart).Milliseconds(), perf.finalise())
		finishReason := "stop"
		if len(toolCalls) > 0 {
			finishReason = "tool_calls"
		}
		chunk := map[string]interface{}{
			"id":      chatID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]interface{}{{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": finishReason,
			}},
			"usage": map[string]int{
				"prompt_tokens":     inputTokens,
				"completion_tokens": outputTokens,
				"total_tokens":      inputTokens + outputTokens,
			},
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", string(data))
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return
	}

	if lastErr == nil {
		h.sendOpenAIError(w, 503, "server_error", "No available accounts")
		return
	}

	h.recordFailureWithDetails(w, "openai", model, "", lastErr)
	h.sendOpenAIError(w, 500, "server_error", lastErr.Error())
}

// handleOpenAINonStream OpenAI 非流式响应
func (h *Handler) handleOpenAINonStream(w http.ResponseWriter, req *OpenAIRequest, model string, thinking bool, estimatedInputTokens int, apiKeyID string, affinityKey string) {
	excluded := make(map[string]bool)
	var lastErr error
	reqStart := time.Now()
	perf := newPerfTracker(reqStart)

	for attempt := 0; attempt < maxAccountRetryAttempts; attempt++ {
		account := h.pickAccount(affinityKey, model, excluded, capabilityFilter(capOpenAIChat))
		if account == nil {
			break
		}
		if err := h.ensureValidToken(account); err != nil {
			lastErr = err
			excluded[account.ID] = true
			h.handleModelFailure(account, model, err)
			continue
		}

		var content string
		var reasoningContent string
		var toolUses []ToolUse
		var inputTokens, outputTokens int
		var credits float64
		var realInputTokens int

		callback := &StreamCallback{
			OnText: func(text string, isThinking bool) {
				perf.markFirstByte()
				perf.addTokens(text)
				if isThinking {
					reasoningContent += text
				} else {
					content += text
				}
			},
			OnToolUse: func(tu ToolUse) {
				perf.markFirstByte()
				toolUses = append(toolUses, tu)
			},
			OnComplete: func(inTok, outTok int) {
				perf.setFinalTokens(outTok)
				inputTokens = inTok
				outputTokens = outTok
			},
			OnCredits: func(c float64) { credits = c },
			OnContextUsage: func(pct float64) {
				realInputTokens = int(pct * float64(getContextWindowSize(model)) / 100.0)
			},
		}

		err := callUpstreamFromOpenAI(account, req, thinking, callback)
		if err != nil {
			lastErr = err
			excluded[account.ID] = true
			h.handleModelFailure(account, model, err)
			continue
		}

		finalContent, extractedReasoning := extractThinkingFromContent(content)
		if thinking && reasoningContent == "" && extractedReasoning != "" {
			reasoningContent = extractedReasoning
		} else if !thinking {
			reasoningContent = ""
		}

		if realInputTokens > 0 {
			inputTokens = realInputTokens
		} else if inputTokens <= 0 {
			inputTokens = estimatedInputTokens
		}
		outputTokens = estimateOpenAIOutputTokens(finalContent, reasoningContent, toolUses)

		h.recordSuccessForApiKey(apiKeyID, inputTokens, outputTokens, credits)
		h.pool.RecordSuccess(account.ID)
		h.pool.UpdateStats(account.ID, inputTokens+outputTokens, credits)
		h.recordSuccessLog(w, "openai", model, account.ID, inputTokens+outputTokens, credits, time.Since(reqStart).Milliseconds(), perf.finalise())

		thinkingFormat := config.GetThinkingConfig().OpenAIFormat
		resp := KiroToOpenAIResponseWithReasoning(finalContent, reasoningContent, toolUses, inputTokens, outputTokens, model, thinkingFormat)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)
		return
	}

	if lastErr == nil {
		h.sendOpenAIError(w, 503, "server_error", "No available accounts")
		return
	}

	h.recordFailureWithDetails(w, "openai", model, "", lastErr)
	h.sendOpenAIError(w, 500, "server_error", lastErr.Error())
}

func (h *Handler) sendOpenAIError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	})
}

// ensureValidToken 确保 token 有效
func (h *Handler) ensureValidToken(account *config.Account) error {
	// API-key and access-token-only CodeBuddy accounts cannot be refreshed;
	// Keycloak offline-token imports can.
	if isCodeBuddyAccount(account) && !auth.CodeBuddyRefreshable(account) {
		return nil
	}
	if account.ExpiresAt == 0 || time.Now().Unix() < account.ExpiresAt-tokenRefreshSkewSeconds {
		return nil
	}

	h.tokenRefreshMu.Lock()
	defer h.tokenRefreshMu.Unlock()

	// Another concurrent request may have refreshed this account while we waited.
	if latest := h.pool.GetByID(account.ID); latest != nil {
		account.AccessToken = latest.AccessToken
		account.RefreshToken = latest.RefreshToken
		account.ExpiresAt = latest.ExpiresAt
		account.ProfileArn = latest.ProfileArn
		if account.ExpiresAt == 0 || time.Now().Unix() < account.ExpiresAt-tokenRefreshSkewSeconds {
			return nil
		}
	}

	accessToken, refreshToken, expiresAt, profileArn, err := auth.RefreshToken(account)
	if err != nil {
		return err
	}

	// 更新内存
	h.pool.UpdateToken(account.ID, accessToken, refreshToken, expiresAt)
	account.AccessToken = accessToken
	if refreshToken != "" {
		account.RefreshToken = refreshToken
	}
	account.ExpiresAt = expiresAt
	if profileArn != "" {
		account.ProfileArn = profileArn
		config.UpdateAccountProfileArn(account.ID, profileArn)
	}

	// 持久化
	config.UpdateAccountToken(account.ID, accessToken, refreshToken, expiresAt)
	if isCodexAccount(account) && account.UserId != "" {
		config.UpdateAccountUserId(account.ID, account.UserId)
	}

	return nil
}

// ==================== 管理 API ====================

func (h *Handler) handleAdminAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/api")

	// First-run setup endpoints bypass the password gate BUT only function while
	// the instance is unconfigured (empty password). This replaces the old
	// "changeme" default: a fresh deploy has no password and the admin UI forces
	// the setup screen. setup/status is always readable so the UI can branch.
	if path == "/setup/status" && r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{"configured": config.IsConfigured()})
		return
	}
	if path == "/setup" && r.Method == "POST" {
		h.apiCompleteSetup(w, r)
		return
	}

	// 验证密码 — constant-time to avoid leaking length/prefix via comparison timing.
	password := r.Header.Get("X-Admin-Password")
	if password == "" {
		cookie, _ := r.Cookie("admin_password")
		if cookie != nil {
			password = cookie.Value
		}
	}

	if !config.IsConfigured() {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Setup required", "setupRequired": "true"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(config.GetPassword())) != 1 {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Provider auth/import endpoints dispatch through the providers route
	// registry; each provider package registers its own routes in init().
	if fn, ok := providers.AdminRoute(r.Method, path); ok {
		fn(h, w, r)
		return
	}

	switch {
	case path == "/accounts" && r.Method == "GET":
		h.apiGetAccounts(w, r)
	case path == "/accounts" && r.Method == "POST":
		h.apiAddAccount(w, r)
	case path == "/accounts/batch" && r.Method == "POST":
		h.apiBatchAccounts(w, r)
	// models/refresh 必须在通用 /refresh 前匹配，否则会被误拦截
	case path == "/accounts/models/refresh" && r.Method == "POST":
		h.apiRefreshAllAccountsModels(w, r)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/models/refresh") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/models/refresh")
		h.apiRefreshAccountModels(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/refresh") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/refresh")
		h.apiRefreshAccount(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/test") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/test")
		h.apiTestAccount(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/models/cached") && r.Method == "GET":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/models/cached")
		h.apiGetAccountModelsCached(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/models") && r.Method == "GET":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/models")
		h.apiGetAccountModels(w, r, id)

	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/overage") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/overage")
		h.apiSetAccountOverage(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/overage") && r.Method == "GET":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/overage")
		h.apiGetAccountOverage(w, r, id)

	case strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/full") && r.Method == "GET":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/accounts/"), "/full")
		h.apiGetAccountFull(w, r, id)
	case strings.HasPrefix(path, "/accounts/") && r.Method == "DELETE":
		h.apiDeleteAccount(w, r, strings.TrimPrefix(path, "/accounts/"))
	case strings.HasPrefix(path, "/accounts/") && r.Method == "PUT":
		h.apiUpdateAccount(w, r, strings.TrimPrefix(path, "/accounts/"))
	case path == "/auth/credentials" && r.Method == "POST":
		h.apiImportCredentials(w, r)
	case path == "/status" && r.Method == "GET":
		h.apiGetStatus(w, r)
	case path == "/settings" && r.Method == "GET":
		h.apiGetSettings(w, r)
	case path == "/settings" && r.Method == "POST":
		h.apiUpdateSettings(w, r)
	case path == "/stats" && r.Method == "GET":
		h.apiGetStats(w, r)
	case path == "/stats/reset" && r.Method == "POST":
		h.apiResetStats(w, r)
	case path == "/logs" && r.Method == "GET":
		h.apiGetLogs(w, r)
	case path == "/logs" && r.Method == "DELETE":
		h.apiClearLogs(w, r)
	case path == "/generate-machine-id" && r.Method == "GET":
		h.apiGenerateMachineId(w, r)
	case path == "/thinking" && r.Method == "GET":
		h.apiGetThinkingConfig(w, r)
	case path == "/thinking" && r.Method == "POST":
		h.apiUpdateThinkingConfig(w, r)
	case path == "/endpoint" && r.Method == "GET":
		h.apiGetEndpointConfig(w, r)
	case path == "/endpoint" && r.Method == "POST":
		h.apiUpdateEndpointConfig(w, r)
	case path == "/proxy" && r.Method == "GET":
		h.apiGetProxy(w, r)
	case path == "/proxy" && r.Method == "POST":
		h.apiUpdateProxy(w, r)
	case path == "/relay" && r.Method == "GET":
		h.apiGetRelay(w, r)
	case path == "/relay" && r.Method == "POST":
		h.apiUpdateRelay(w, r)
	case path == "/relay/test" && r.Method == "POST":
		h.apiTestRelay(w, r)
	case path == "/relay/source" && r.Method == "GET":
		h.apiGetRelaySource(w, r)
	case path == "/auto-route" && r.Method == "GET":
		h.apiGetAutoRoute(w, r)
	case path == "/auto-route" && r.Method == "POST":
		h.apiUpdateAutoRoute(w, r)
	case path == "/auto-route/decisions" && r.Method == "GET":
		h.apiGetAutoRouteDecisions(w, r)
	case path == "/metrics" && r.Method == "GET":
		h.apiGetMetrics(w, r)
	case path == "/warmup/status" && r.Method == "GET":
		h.apiWarmupStatus(w, r)
	case path == "/warmup" && r.Method == "POST":
		h.apiRunWarmup(w, r)
	case path == "/prompt-filter" && r.Method == "GET":
		h.apiGetPromptFilter(w, r)
	case path == "/prompt-filter" && r.Method == "POST":
		h.apiUpdatePromptFilter(w, r)
	case path == "/version" && r.Method == "GET":
		h.apiGetVersion(w, r)
	case path == "/export" && r.Method == "POST":
		h.apiExportAccounts(w, r)
	case path == "/api-keys" && r.Method == "GET":
		h.apiListApiKeys(w, r)
	case path == "/api-keys" && r.Method == "POST":
		h.apiCreateApiKey(w, r)
	case strings.HasPrefix(path, "/api-keys/") && strings.HasSuffix(path, "/reset-usage") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api-keys/"), "/reset-usage")
		h.apiResetApiKeyUsage(w, r, id)
	case strings.HasPrefix(path, "/api-keys/") && r.Method == "GET":
		h.apiGetApiKey(w, r, strings.TrimPrefix(path, "/api-keys/"))
	case strings.HasPrefix(path, "/api-keys/") && r.Method == "PUT":
		h.apiUpdateApiKey(w, r, strings.TrimPrefix(path, "/api-keys/"))
	case strings.HasPrefix(path, "/api-keys/") && r.Method == "DELETE":
		h.apiDeleteApiKey(w, r, strings.TrimPrefix(path, "/api-keys/"))
	default:
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Not Found"})
	}
}

func (h *Handler) apiGetAccounts(w http.ResponseWriter, r *http.Request) {
	accounts := config.GetAccounts()
	poolAccounts := h.pool.GetAllAccounts()

	// 合并运行时统计
	statsMap := make(map[string]config.Account)
	for _, a := range poolAccounts {
		statsMap[a.ID] = a
	}

	// 隐藏敏感信息
	result := make([]map[string]interface{}, len(accounts))
	for i, a := range accounts {
		// 获取运行时统计
		stats := statsMap[a.ID]

		displayProvider := a.Provider
		if a.AuthMethod == "external_idp" {
			displayProvider = "AzureAD"
		}

		result[i] = map[string]interface{}{
			"id":                a.ID,
			"email":             a.Email,
			"userId":            a.UserId,
			"nickname":          a.Nickname,
			"authMethod":        a.AuthMethod,
			"provider":          displayProvider,
			"region":            a.Region,
			"enabled":           a.Enabled,
			"banStatus":         a.BanStatus,
			"banReason":         a.BanReason,
			"banTime":           a.BanTime,
			"expiresAt":         a.ExpiresAt,
			"hasToken":          a.AccessToken != "",
			"machineId":         a.MachineId,
			"weight":            a.Weight,
			"probeModel":        a.ProbeModel,
			"extraModels":       a.ExtraModels,
			"overageStatus":     a.OverageStatus,
			"overageCapability": a.OverageCapability,
			"overageCap":        a.OverageCap,
			"overageRate":       a.OverageRate,
			"currentOverages":   a.CurrentOverages,
			"overageCheckedAt":  a.OverageCheckedAt,
			"proxyURL":          a.ProxyURL,
			"relayURL":          a.RelayURL,
			"hasRelaySecret":    a.RelaySecret != "",
			"subscriptionType":  a.SubscriptionType,
			"subscriptionTitle": a.SubscriptionTitle,
			"daysRemaining":     a.DaysRemaining,
			"usageCurrent":      a.UsageCurrent,
			"usageLimit":        a.UsageLimit,
			"usagePercent":      a.UsagePercent,
			"nextResetDate":     a.NextResetDate,
			"lastRefresh":       a.LastRefresh,
			"trialUsageCurrent": a.TrialUsageCurrent,
			"trialUsageLimit":   a.TrialUsageLimit,
			"trialUsagePercent": a.TrialUsagePercent,
			"trialStatus":       a.TrialStatus,
			"trialExpiresAt":    a.TrialExpiresAt,
			"requestCount":      stats.RequestCount,
			"errorCount":        stats.ErrorCount,
			"totalTokens":       stats.TotalTokens,
			"totalCredits":      stats.TotalCredits,
			"lastUsed":          stats.LastUsed,
			"warmupStatus":      a.WarmupStatus,
			"warmupError":       a.WarmupError,
			"lastWarmup":        a.LastWarmup,
		}
	}
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) apiAddAccount(w http.ResponseWriter, r *http.Request) {
	var account config.Account
	if err := json.NewDecoder(r.Body).Decode(&account); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if account.ID == "" {
		account.ID = auth.GenerateAccountID()
	}
	if account.Region == "" {
		account.Region = "us-east-1"
	}

	if err := config.AddAccount(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.pool.Reload()
	// 新账号若已启用且有 token，立即拉取并缓存模型列表
	if account.Enabled && account.AccessToken != "" {
		go func(acc config.Account) {
			if err := h.fetchAndCacheAccountModels(&acc); err != nil {
				logger.Warnf("[ModelsCache] Auto-refresh failed for new account %s: %v", acc.Email, err)
			}
		}(account)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": account.ID})
}

func (h *Handler) apiDeleteAccount(w http.ResponseWriter, r *http.Request, id string) {
	if err := config.DeleteAccount(id); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.pool.Reload()
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiUpdateAccount(w http.ResponseWriter, r *http.Request, id string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// 获取现有账号
	accounts := config.GetAccounts()
	var existing *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			existing = &accounts[i]
			break
		}
	}
	if existing == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}

	// 只更新传入的字段
	oldEnabled := existing.Enabled
	if v, ok := updates["enabled"].(bool); ok {
		existing.Enabled = v
	}
	if v, ok := updates["nickname"].(string); ok {
		existing.Nickname = v
	}
	if v, ok := updates["machineId"].(string); ok {
		existing.MachineId = v
	}
	if v, ok := updates["weight"].(float64); ok {
		existing.Weight = int(v)
	}
	if v, ok := updates["probeModel"].(string); ok {
		existing.ProbeModel = strings.TrimSpace(v)
	}
	if v, ok := updates["extraModels"].([]interface{}); ok {
		existing.ExtraModels = existing.ExtraModels[:0:0]
		for _, item := range v {
			if id, ok := item.(string); ok && strings.TrimSpace(id) != "" {
				existing.ExtraModels = append(existing.ExtraModels, strings.TrimSpace(id))
			}
		}
	}
	if v, ok := updates["proxyURL"].(string); ok {
		if v != "" && !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") && !strings.HasPrefix(v, "socks5://") && !strings.HasPrefix(v, "socks5h://") {
			http.Error(w, `{"error":"invalid proxyURL"}`, http.StatusBadRequest)
			return
		}
		existing.ProxyURL = v
		if v != "" {
			existing.RelayURL, existing.RelaySecret = "", ""
		}
	}
	if v, ok := updates["relayURL"].(string); ok {
		if v != "" && !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			http.Error(w, `{"error":"invalid relayURL"}`, http.StatusBadRequest)
			return
		}
		existing.RelayURL = v
		if v != "" {
			existing.ProxyURL = ""
		}
		if v == "" {
			existing.RelaySecret = ""
		}
	}
	if v, ok := updates["relaySecret"].(string); ok && v != "" {
		existing.RelaySecret = v
	}

	if err := config.UpdateAccount(id, *existing); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.pool.Reload()
	// 账号从禁用→启用时，自动拉取并缓存模型列表
	if !oldEnabled && existing.Enabled && existing.AccessToken != "" {
		go func(acc config.Account) {
			if err := h.fetchAndCacheAccountModels(&acc); err != nil {
				logger.Warnf("[ModelsCache] Auto-refresh failed for re-enabled account %s: %v", acc.Email, err)
			}
		}(*existing)
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// apiGetAccountOverage 拉取并返回单个账号的上游 Overages 状态。
// 同步把结果写回 config.json 缓存，确保 UI 与持久化一致。
func (h *Handler) apiGetAccountOverage(w http.ResponseWriter, r *http.Request, id string) {
	accounts := config.GetAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}

	snap, err := FetchOverageStatus(account)
	if err != nil {
		w.WriteHeader(502)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if persistErr := PersistOverageSnapshot(id, snap); persistErr != nil {
		logger.Warnf("[Overage] persist GET overage failed for %s: %v", account.Email, persistErr)
	}
	h.pool.Reload()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":           true,
		"overageStatus":     snap.Status,
		"overageCapability": snap.Capability,
		"subscriptionTitle": snap.SubscriptionTitle,
		"overageCap":        snap.OverageCap,
		"overageRate":       snap.OverageRate,
		"currentOverages":   snap.CurrentOverages,
		"overageCheckedAt":  snap.CheckedAt,
	})
}

// apiSetAccountOverage 翻转单个账号的上游 Overages 开关，并刷新缓存。
// Body: {"enabled": true|false}
func (h *Handler) apiSetAccountOverage(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	accounts := config.GetAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}

	snap, err := SetOverageStatus(account, body.Enabled)
	if err != nil {
		w.WriteHeader(502)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if persistErr := PersistOverageSnapshot(id, snap); persistErr != nil {
		logger.Warnf("[Overage] persist SET overage failed for %s: %v", account.Email, persistErr)
	}
	h.pool.Reload()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":           true,
		"overageStatus":     snap.Status,
		"overageCapability": snap.Capability,
		"subscriptionTitle": snap.SubscriptionTitle,
		"overageCap":        snap.OverageCap,
		"overageRate":       snap.OverageRate,
		"currentOverages":   snap.CurrentOverages,
		"overageCheckedAt":  snap.CheckedAt,
	})
}

// apiBatchAccounts 批量操作账号（启用/禁用/刷新）
func (h *Handler) apiBatchAccounts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"` // "enable", "disable", "refresh"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if len(req.IDs) == 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "No account IDs provided"})
		return
	}

	switch req.Action {
	case "enable", "disable":
		enabled := req.Action == "enable"
		accounts := config.GetAccounts()
		idSet := make(map[string]bool)
		for _, id := range req.IDs {
			idSet[id] = true
		}
		var toRefreshModels []config.Account
		for _, a := range accounts {
			if idSet[a.ID] {
				// 记录本次从禁用→启用、且有 token 的账号
				if enabled && !a.Enabled && a.AccessToken != "" {
					toRefreshModels = append(toRefreshModels, a)
				}
				a.Enabled = enabled
				if enabled && a.BanStatus != "" && a.BanStatus != "ACTIVE" {
					a.BanStatus = "ACTIVE"
					a.BanReason = ""
					a.BanTime = 0
				}
				config.UpdateAccount(a.ID, a)
			}
		}
		h.pool.Reload()
		// 为本次新启用的账号异步拉取模型缓存
		for _, acc := range toRefreshModels {
			go func(a config.Account) {
				a.Enabled = true
				if err := h.fetchAndCacheAccountModels(&a); err != nil {
					logger.Warnf("[ModelsCache] Auto-refresh failed for batch-enabled account %s: %v", a.Email, err)
				}
			}(acc)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "count": len(req.IDs)})

	case "refresh":
		successCount := 0
		failCount := 0
		for _, id := range req.IDs {
			accounts := config.GetAccounts()
			var account *config.Account
			for i := range accounts {
				if accounts[i].ID == id {
					account = &accounts[i]
					break
				}
			}
			if account == nil {
				failCount++
				continue
			}
			// 刷新 token
			if account.RefreshToken != "" {
				if newAccess, newRefresh, newExpires, profileArn, err := auth.RefreshToken(account); err == nil {
					account.AccessToken = newAccess
					if newRefresh != "" {
						account.RefreshToken = newRefresh
					}
					account.ExpiresAt = newExpires
					config.UpdateAccountToken(id, newAccess, newRefresh, newExpires)
					if profileArn != "" {
						account.ProfileArn = profileArn
						config.UpdateAccountProfileArn(id, profileArn)
					}
					if isCodexAccount(account) && account.UserId != "" {
						config.UpdateAccountUserId(id, account.UserId)
					}
					h.pool.UpdateToken(id, newAccess, newRefresh, newExpires)
				}
			}
			// 刷新账户信息
			info, err := RefreshAccountInfo(account)
			if err != nil {
				failCount++
				continue
			}
			config.UpdateAccountInfo(id, *info)
			successCount++
		}
		h.pool.Reload()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"refreshed": successCount,
			"failed":    failCount,
		})

	default:
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid action: " + req.Action})
	}
}

// apiCompleteSetup handles the first-run initial-setup submission: it sets the
// admin password on an unconfigured instance. config.CompleteSetup refuses once a
// password already exists, so this unauthenticated endpoint cannot overwrite the
// credentials of a configured instance (it only closes the fresh-install window).
func (h *Handler) apiCompleteSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if config.IsConfigured() {
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{"error": "Already configured"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if len(strings.TrimSpace(req.Password)) < 8 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 8 characters"})
		return
	}
	if err := config.CompleteSetup(req.Password); err != nil {
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (h *Handler) apiImportCredentials(w http.ResponseWriter, r *http.Request) {
	// Cap the body: accessToken becomes attacker-influenced input that is base64- and
	// JSON-decoded twice (issuerFromAccessTokenJWT / ExpFromAccessTokenJWT). Without a
	// limit an oversized token is a memory-amplification DoS. Mirrors the io.LimitReader
	// guard on outbound IdP responses in auth/kiro_sso.go's oidcDiscover.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		APIKey       string `json:"apiKey"`
		Label        string `json:"label"`
		Variant      string `json:"variant"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		AuthMethod   string `json:"authMethod"`
		Provider     string `json:"provider"`
		Region       string `json:"region"`
		// external_idp (enterprise SSO / Azure AD) refresh material.
		TokenEndpoint string `json:"tokenEndpoint"`
		IssuerURL     string `json:"issuerUrl"`
		Scopes        string `json:"scopes"`
		// Optional identity preservation when pasting a full account record.
		ID         string `json:"id"`
		Email      string `json:"email"`
		ProfileArn string `json:"profileArn"`
		// userId (account-level in Kiro Account Manager exports) embeds the Azure
		// tenant, from which tokenEndpoint/issuerUrl/scopes are derived when missing.
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if strings.TrimSpace(req.APIKey) != "" || strings.Contains(strings.ToLower(req.AuthMethod+" "+req.Provider+" "+req.Variant), "codebuddy") {
		apiKey := strings.TrimSpace(req.APIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(req.AccessToken)
		}
		if apiKey == "" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "apiKey is required"})
			return
		}
		variant := strings.ToLower(strings.TrimSpace(req.Variant + " " + req.Region + " " + req.AuthMethod + " " + req.Provider))
		provider := "CodeBuddy"
		authMethod := "codebuddy"
		region := "global"
		if strings.Contains(variant, "cn") || strings.Contains(variant, "china") || strings.Contains(variant, "tencent") {
			provider = "CodeBuddy CN"
			authMethod = "codebuddy-cn"
			region = "cn"
		}
		label := strings.TrimSpace(req.Label)
		if label == "" {
			label = strings.TrimSpace(req.Email)
		}
		if label == "" {
			label = provider + " API Key"
		}
		account := config.Account{
			ID:           auth.GenerateAccountID(),
			Email:        label,
			Nickname:     label,
			AccessToken:  apiKey,
			RefreshToken: apiKey,
			AuthMethod:   authMethod,
			Provider:     provider,
			Region:       region,
			Enabled:      true,
			MachineId:    config.GenerateMachineId(),
		}
		if err := config.AddAccount(account); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		h.pool.Reload()
		if err := h.fetchAndCacheAccountModels(&account); err != nil {
			logger.Warnf("[CodeBuddy] Model cache refresh failed for %s: %v", account.Email, err)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "account": map[string]interface{}{"id": account.ID, "email": account.Email, "authMethod": account.AuthMethod, "provider": account.Provider}})
		return
	}

	if req.RefreshToken == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "refreshToken is required"})
		return
	}

	// 设置默认值
	if req.Region == "" {
		req.Region = "us-east-1"
	}
	// 标准化 authMethod。external_idp 必须先于 clientId+clientSecret→idc 的推断被识别
	//（external_idp 带 clientId 但没有 clientSecret），否则会被误判成 social 而 refresh 到错误端点。
	req.AuthMethod = normalizeImportAuthMethod(req.AuthMethod, req.ClientID, req.ClientSecret, req.TokenEndpoint)

	// Resolve Azure endpoints from userId (Kiro export, account level) or the
	// accessToken JWT issuer (bare blobs: clientId + token only). A derivation that
	// also clears the allow-list is itself proof the credential is external_idp —
	// IdC/social access tokens are not microsoftonline JWTs, so a bare IdC blob (its
	// iss is an AWS host) won't clear the list and won't be misclassified.
	derivedTE, derivedIss, derivedSc := auth.DeriveExternalIdpEndpoints(req.UserID, req.ClientID, req.AccessToken)
	if derivedTE != "" && auth.ValidateExternalIdpEndpoint(derivedTE) == nil && req.AuthMethod != "external_idp" {
		req.AuthMethod = "external_idp"
	}

	// external_idp 的 tokenEndpoint 是用户可填的新信任边界：必须经 allow-list 校验，
	// 否则一份不信任的 credential JSON 可指向内网/攻击者主机，导致 refresh token 被外泄。
	if req.AuthMethod == "external_idp" {
		// Kiro Account Manager exports and bare blobs omit tokenEndpoint/issuerUrl/
		// scopes; fill them from the derived (userId or accessToken-JWT) tenant.
		if req.TokenEndpoint == "" {
			req.TokenEndpoint = derivedTE
		}
		if req.IssuerURL == "" {
			req.IssuerURL = derivedIss
		}
		if req.Scopes == "" {
			req.Scopes = derivedSc
		}
		if req.ClientID == "" || req.TokenEndpoint == "" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "external_idp requires clientId and tokenEndpoint (or userId/accessToken to derive it)"})
			return
		}
		if err := auth.ValidateExternalIdpEndpoint(req.TokenEndpoint); err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "external IdP endpoint rejected: " + err.Error()})
			return
		}
		if req.IssuerURL != "" {
			if err := auth.ValidateExternalIdpEndpoint(req.IssuerURL); err != nil {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "external IdP issuer rejected: " + err.Error()})
				return
			}
		}
	}

	// Resolve the access token to persist. For external_idp we prefer TRUST-ON-IMPORT:
	// when the pasted JSON carries an Azure AD access token (a JWT with a real exp),
	// persist it directly WITHOUT a live refresh round-trip. The JSON can then be
	// imported repeatedly / into multiple instances without each import consuming
	// (rotating) the refresh token, and without requiring egress to Microsoft at
	// import time. The runtime background refresh (backgroundRefresh /
	// ensureValidToken) renews it later when the account is actually used. Falls
	// back to refresh-at-import for idc/social and for external_idp credentials
	// carrying only a refreshToken (so the regression gate — reject when refresh
	// fails — still holds there).
	var (
		accessToken string
		expiresAt   int64
		profileArn  string
	)
	email := req.Email
	if req.AuthMethod == "external_idp" && req.AccessToken != "" {
		if exp := auth.ExpFromAccessTokenJWT(req.AccessToken); exp > 0 {
			// The exp comes from an UNVERIFIED JWT: clamp it to a sane horizon so a
			// crafted far-future exp cannot pin a dead token as forever-valid (the
			// background refresh would otherwise never renew it). Azure AD access
			// tokens live ~1h; 24h is a generous ceiling.
			if maxExp := time.Now().Add(24 * time.Hour).Unix(); exp > maxExp {
				exp = maxExp
			}
			accessToken = req.AccessToken
			expiresAt = exp
			profileArn = req.ProfileArn
		}
	}
	if accessToken == "" {
		tempAccount := &config.Account{
			RefreshToken:  req.RefreshToken,
			ClientID:      req.ClientID,
			ClientSecret:  req.ClientSecret,
			AuthMethod:    req.AuthMethod,
			Region:        req.Region,
			TokenEndpoint: req.TokenEndpoint,
			Scopes:        req.Scopes,
		}
		a, newRT, ea, newPA, err := auth.RefreshToken(tempAccount)
		if err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "Token refresh failed: " + err.Error()})
			return
		}
		accessToken = a
		expiresAt = ea
		profileArn = newPA
		if newRT != "" {
			req.RefreshToken = newRT
		}
		if fetchedEmail, _, _ := auth.GetUserInfo(accessToken); fetchedEmail != "" {
			email = fetchedEmail
		}
	}
	if profileArn == "" {
		profileArn = req.ProfileArn // external_idp refresh returns no profileArn
	}

	// 创建账号
	provider := req.Provider
	if req.AuthMethod == "external_idp" {
		// Some frontend import paths used to default an Azure/Entra credential with
		// no clientSecret to provider=Google before the backend derived
		// external_idp from userId/accessToken. Once backend has classified it as
		// external_idp, force the display provider to AzureAD.
		provider = "AzureAD"
	}
	// Reuse a pasted record's id when it does not collide; otherwise mint a fresh
	// one so re-importing a backup never creates a duplicate entry.
	id := req.ID
	if id == "" || config.AccountIDExists(id) {
		id = auth.GenerateAccountID()
	}
	account := config.Account{
		ID:            id,
		Email:         email,
		AccessToken:   accessToken,
		RefreshToken:  req.RefreshToken,
		ClientID:      req.ClientID,
		ClientSecret:  req.ClientSecret,
		AuthMethod:    req.AuthMethod,
		Provider:      provider,
		Region:        req.Region,
		ExpiresAt:     expiresAt,
		Enabled:       true,
		MachineId:     config.GenerateMachineId(),
		ProfileArn:    profileArn,
		TokenEndpoint: req.TokenEndpoint,
		IssuerURL:     req.IssuerURL,
		Scopes:        req.Scopes,
	}

	if err := config.AddAccount(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.pool.Reload()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"account": map[string]interface{}{
			"id":    account.ID,
			"email": account.Email,
		},
	})
}

// externalIdpAuthMethodAliases are lower-cased authMethod values (or Kiro Account
// Manager provider labels) that mean "external IdP / enterprise SSO" and must
// normalize to "external_idp".
var externalIdpAuthMethodAliases = map[string]bool{
	"external_idp": true,
	"azuread":      true,
	"azure":        true,
	"entra":        true,
	"entra-id":     true,
	"entra_id":     true,
	"microsoft":    true,
	"m365":         true,
	"office365":    true,
	"external":     true,
}

// normalizeImportAuthMethod maps a pasted credential JSON's authMethod (plus its
// clientId/clientSecret/tokenEndpoint) onto one of the three canonical methods
// ("external_idp" | "idc" | "social"). external_idp MUST be detected before the
// clientId+clientSecret→idc inference, because external_idp accounts carry clientId
// but NO clientSecret, so the old default branch misclassified them as "social" and
// refresh hit the wrong endpoint.
//
// It preserves the pre-existing idc/social heuristics:
//   - empty authMethod + clientId present             -> idc
//   - empty authMethod, no clientId                   -> social
//   - "enterprise" (Kiro Account Manager IdC label)   -> idc
//   - unrecognized non-empty + clientId+clientSecret  -> idc, else social
func normalizeImportAuthMethod(authMethod, clientID, clientSecret, tokenEndpoint string) string {
	am := strings.ToLower(strings.TrimSpace(authMethod))
	switch {
	case externalIdpAuthMethodAliases[am]:
		return "external_idp"
	case am == "social" || am == "google" || am == "github":
		return "social"
	case am == "idc" || am == "builderid" || am == "enterprise":
		return "idc"
	case tokenEndpoint != "":
		// Infer external_idp from a tokenEndpoint only when authMethod does not
		// explicitly say otherwise — a stray tokenEndpoint key in a pasted social/
		// idc record must not silently flip the account to external_idp.
		return "external_idp"
	}
	if am == "" {
		if clientID != "" {
			return "idc"
		}
		return "social"
	}
	if clientID != "" && clientSecret != "" {
		return "idc"
	}
	return "social"
}

func (h *Handler) apiGetStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":         config.Version,
		"accounts":        h.pool.Count(),
		"available":       h.pool.AvailableCount(),
		"totalRequests":   atomic.LoadInt64(&h.totalRequests),
		"successRequests": atomic.LoadInt64(&h.successRequests),
		"failedRequests":  atomic.LoadInt64(&h.failedRequests),
		"totalTokens":     atomic.LoadInt64(&h.totalTokens),
		"totalCredits":    h.getCredits(),
		"uptime":          time.Now().Unix() - h.startTime,
	})
}

func (h *Handler) apiGetSettings(w http.ResponseWriter, r *http.Request) {
	warmupProbe, warmupRecover := config.GetWarmupOptions()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"apiKey":                config.GetApiKey(),
		"requireApiKey":         config.IsApiKeyRequired(),
		"port":                  config.GetPort(),
		"host":                  config.GetHost(),
		"allowOverUsage":        config.GetAllowOverUsage(),
		"logLevel":              config.GetLogLevel(),
		"accountRefreshMinutes": config.GetAccountRefreshMinutes(),
		"warmupProbe":           warmupProbe,
		"warmupRecover":         warmupRecover,
		"testModel":             config.GetTestModel(),
		"customModelIds":        map[string][]string{"codebuddy": config.GetCustomModelIDs("codebuddy")},
	})
}

func (h *Handler) apiGetPromptFilter(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(config.GetPromptFilterConfig())
}

func (h *Handler) apiUpdatePromptFilter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FilterClaudeCode      *bool                      `json:"filterClaudeCode,omitempty"`
		FilterEnvNoise        *bool                      `json:"filterEnvNoise,omitempty"`
		FilterStripBoundaries *bool                      `json:"filterStripBoundaries,omitempty"`
		Rules                 *[]config.PromptFilterRule `json:"rules,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Read current config to fill in any fields not provided in the request.
	current := config.GetPromptFilterConfig()
	fcc := current.FilterClaudeCode
	fen := current.FilterEnvNoise
	fsb := current.FilterStripBoundaries
	rules := current.Rules
	if req.FilterClaudeCode != nil {
		fcc = *req.FilterClaudeCode
	}
	if req.FilterEnvNoise != nil {
		fen = *req.FilterEnvNoise
	}
	if req.FilterStripBoundaries != nil {
		fsb = *req.FilterStripBoundaries
	}
	if req.Rules != nil {
		rules = *req.Rules
	}
	if err := config.UpdatePromptFilterConfig(fcc, fen, fsb, rules); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ApiKey                *string              `json:"apiKey,omitempty"`
		RequireApiKey         *bool                `json:"requireApiKey,omitempty"`
		Password              string               `json:"password,omitempty"`
		AllowOverUsage        *bool                `json:"allowOverUsage,omitempty"`
		LogLevel              *string              `json:"logLevel,omitempty"`
		AccountRefreshMinutes *int                 `json:"accountRefreshMinutes,omitempty"`
		WarmupProbe           *bool                `json:"warmupProbe,omitempty"`
		WarmupRecover         *bool                `json:"warmupRecover,omitempty"`
		TestModel             *string              `json:"testModel,omitempty"`
		CustomModelIDs        *map[string][]string `json:"customModelIds,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.LogLevel != nil {
		lvl, ok := logger.ParseLevel(*req.LogLevel)
		if !ok {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "logLevel must be debug, info, warn or error"})
			return
		}
		if err := config.UpdateLogLevel(*req.LogLevel); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		logger.SetLevel(lvl)
	}
	if req.TestModel != nil {
		if err := config.UpdateTestModel(*req.TestModel); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}
	if req.CustomModelIDs != nil {
		for provider, ids := range *req.CustomModelIDs {
			if err := config.UpdateCustomModelIDs(provider, ids); err != nil {
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
		}
		h.pool.Reload()
	}
	if req.WarmupProbe != nil || req.WarmupRecover != nil {
		probe, recover := config.GetWarmupOptions()
		if req.WarmupProbe != nil {
			probe = *req.WarmupProbe
		}
		if req.WarmupRecover != nil {
			recover = *req.WarmupRecover
		}
		if err := config.UpdateWarmupOptions(probe, recover); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}
	if req.AccountRefreshMinutes != nil {
		if *req.AccountRefreshMinutes < 0 || *req.AccountRefreshMinutes > 1440 {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "accountRefreshMinutes must be between 0 (default) and 1440"})
			return
		}
		if err := config.UpdateAccountRefreshMinutes(*req.AccountRefreshMinutes); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}

	if err := config.UpdateSettingsPatch(req.ApiKey, req.RequireApiKey, req.Password); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 更新超额使用设置
	if req.AllowOverUsage != nil {
		if err := config.UpdateAllowOverUsage(*req.AllowOverUsage); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		// Rebuild the pool so over-quota accounts are re-included or dropped immediately.
		h.pool.Reload()
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetStats(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalRequests":   atomic.LoadInt64(&h.totalRequests),
		"successRequests": atomic.LoadInt64(&h.successRequests),
		"failedRequests":  atomic.LoadInt64(&h.failedRequests),
		"totalTokens":     atomic.LoadInt64(&h.totalTokens),
		"totalCredits":    h.getCredits(),
		"uptime":          time.Now().Unix() - h.startTime,
	})
}

func (h *Handler) apiResetStats(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt64(&h.totalRequests, 0)
	atomic.StoreInt64(&h.successRequests, 0)
	atomic.StoreInt64(&h.failedRequests, 0)
	atomic.StoreInt64(&h.totalTokens, 0)
	h.creditsMu.Lock()
	h.totalCredits = 0
	h.creditsMu.Unlock()
	config.UpdateStats(0, 0, 0, 0, 0)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetLogs(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": h.getRequestLogs(),
	})
}

func (h *Handler) apiClearLogs(w http.ResponseWriter, r *http.Request) {
	h.requestLogsMu.Lock()
	h.requestLogs = h.requestLogs[:0]
	h.requestLogsMu.Unlock()
	if rs := config.RequestLogs(); rs != nil {
		if err := rs.Clear(); err != nil {
			logger.Warnf("request log clear failed: %v", err)
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// apiGenerateMachineId 生成新的机器码
func (h *Handler) apiGenerateMachineId(w http.ResponseWriter, r *http.Request) {
	machineId := config.GenerateMachineId()
	json.NewEncoder(w).Encode(map[string]string{"machineId": machineId})
}

// apiTestAccount tests a specific account by sending a real model request through its proxy.
func (h *Handler) apiTestAccount(w http.ResponseWriter, r *http.Request, id string) {
	accounts := config.GetAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}
	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}

	if err := h.ensureValidToken(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": "Token refresh failed: " + err.Error()})
		return
	}

	// Parse test model from request body (optional)
	var req struct {
		Model string `json:"model"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Model == "" {
		req.Model = probeModelFor(account)
	}

	// Build a minimal chat payload
	thinkingCfg := config.GetThinkingConfig()
	actualModel, thinking := ParseModelAndThinking(req.Model, thinkingCfg.Suffix)

	maxTokens := 5
	if isCodeBuddyAccount(account) {
		// CodeBuddy may reject admin smoke tests that set max_tokens explicitly.
		// Omit it for CodeBuddy and let upstream use its default output budget.
		maxTokens = 0
	}
	openaiReq := &OpenAIRequest{
		Model:     actualModel,
		Messages:  []OpenAIMessage{{Role: "user", Content: "say ok"}},
		MaxTokens: maxTokens,
		Stream:    false,
	}
	var content string
	callback := &StreamCallback{
		OnText:         func(text string, isThinking bool) { content += text },
		OnToolUse:      func(tu ToolUse) {},
		OnComplete:     func(inTok, outTok int) {},
		OnError:        func(err error) {},
		OnCredits:      func(c float64) {},
		OnContextUsage: func(pct float64) {},
	}

	err := callUpstreamFromOpenAI(account, openaiReq, thinking, callback)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"reply":   content,
		"model":   req.Model,
	})
}

// apiRefreshAccount 刷新账户信息（使用量、订阅等）
func (h *Handler) apiRefreshAccount(w http.ResponseWriter, r *http.Request, id string) {
	accounts := config.GetAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}

	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}

	// 先尝试刷新 token（不管是否过期，确保 token 有效）
	refreshTokenIfNeeded := func() error {
		if account.RefreshToken == "" {
			return nil
		}
		newAccessToken, newRefreshToken, newExpiresAt, profileArn, err := auth.RefreshToken(account)
		if err != nil {
			return err
		}
		account.AccessToken = newAccessToken
		if newRefreshToken != "" {
			account.RefreshToken = newRefreshToken
		}
		account.ExpiresAt = newExpiresAt
		config.UpdateAccountToken(id, newAccessToken, newRefreshToken, newExpiresAt)
		h.pool.UpdateToken(id, newAccessToken, newRefreshToken, newExpiresAt)
		if profileArn != "" {
			account.ProfileArn = profileArn
			config.UpdateAccountProfileArn(id, profileArn)
		}
		if isCodexAccount(account) && account.UserId != "" {
			config.UpdateAccountUserId(id, account.UserId)
		}
		return nil
	}

	var opts struct {
		ForceToken bool `json:"forceToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&opts)

	// 检查 token 是否快过期，先刷新（或操作者要求强制刷新）
	if opts.ForceToken || (account.ExpiresAt > 0 && time.Now().Unix() > account.ExpiresAt-tokenRefreshSkewSeconds) {
		if err := refreshTokenIfNeeded(); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": "Token refresh failed: " + err.Error()})
			return
		}
	}

	// 获取账户信息
	info, err := RefreshAccountInfo(account)
	if err != nil {
		// 检查是否为封禁相关错误
		errMsg := err.Error()
		if strings.Contains(errMsg, "TEMPORARILY_SUSPENDED") || strings.Contains(errMsg, "Account suspended") {
			// 封禁状态已在 RefreshAccountInfo 中处理，静默返回成功
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Account status updated",
			})
			return
		}

		// 如果是 403/401，说明 token 无效，尝试刷新后重试
		if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "expired") {
			if refreshErr := refreshTokenIfNeeded(); refreshErr == nil {
				// 重试
				info, err = RefreshAccountInfo(account)
				if err != nil {
					// 重试后仍然失败，检查是否为封禁状态
					if strings.Contains(err.Error(), "TEMPORARILY_SUSPENDED") || strings.Contains(err.Error(), "Account suspended") {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"success": true,
							"message": "Account status updated",
						})
						return
					}
				}
			}
		}

		// 其他错误才显示错误信息
		if err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}

	// 保存到配置
	if err := config.UpdateAccountInfo(id, *info); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"info":    info,
	})
}

// apiGetAccountFull 获取单个账号的完整信息（包含敏感字段）
func (h *Handler) apiGetAccountFull(w http.ResponseWriter, r *http.Request, id string) {
	accounts := config.GetAccounts()
	poolAccounts := h.pool.GetAllAccounts()

	// 查找指定账号
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}

	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}

	// 获取运行时统计
	var stats config.Account
	for _, a := range poolAccounts {
		if a.ID == id {
			stats = a
			break
		}
	}

	// 返回完整账号信息（包含敏感字段）
	result := map[string]interface{}{
		"id":           account.ID,
		"email":        account.Email,
		"userId":       account.UserId,
		"nickname":     account.Nickname,
		"accessToken":  account.AccessToken,
		"refreshToken": account.RefreshToken,
		"clientId":     account.ClientID,
		"clientSecret": account.ClientSecret,
		"authMethod":   account.AuthMethod,
		"provider": func() string {
			if account.AuthMethod == "external_idp" {
				return "AzureAD"
			}
			return account.Provider
		}(),
		"region":            account.Region,
		"expiresAt":         account.ExpiresAt,
		"machineId":         account.MachineId,
		"weight":            account.Weight,
		"overageStatus":     account.OverageStatus,
		"overageCapability": account.OverageCapability,
		"overageCap":        account.OverageCap,
		"overageRate":       account.OverageRate,
		"currentOverages":   account.CurrentOverages,
		"overageCheckedAt":  account.OverageCheckedAt,
		"proxyURL":          account.ProxyURL,
		"relayURL":          account.RelayURL,
		"hasRelaySecret":    account.RelaySecret != "",
		"enabled":           account.Enabled,
		"banStatus":         account.BanStatus,
		"banReason":         account.BanReason,
		"banTime":           account.BanTime,
		"subscriptionType":  account.SubscriptionType,
		"subscriptionTitle": account.SubscriptionTitle,
		"daysRemaining":     account.DaysRemaining,
		"usageCurrent":      account.UsageCurrent,
		"usageLimit":        account.UsageLimit,
		"usagePercent":      account.UsagePercent,
		"nextResetDate":     account.NextResetDate,
		"lastRefresh":       account.LastRefresh,
		"trialUsageCurrent": account.TrialUsageCurrent,
		"trialUsageLimit":   account.TrialUsageLimit,
		"trialUsagePercent": account.TrialUsagePercent,
		"trialStatus":       account.TrialStatus,
		"trialExpiresAt":    account.TrialExpiresAt,
		"requestCount":      stats.RequestCount,
		"errorCount":        stats.ErrorCount,
		"totalTokens":       stats.TotalTokens,
		"totalCredits":      stats.TotalCredits,
		"lastUsed":          stats.LastUsed,
	}

	json.NewEncoder(w).Encode(result)
}

// apiGetAccountModels 获取账户可用模型
func (h *Handler) apiGetAccountModels(w http.ResponseWriter, r *http.Request, id string) {
	accounts := config.GetAccounts()
	var account *config.Account
	for i := range accounts {
		if accounts[i].ID == id {
			account = &accounts[i]
			break
		}
	}

	if account == nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not found"})
		return
	}

	models, err := ListAvailableModels(account)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 同步更新路由缓存
	modelIDs := make([]string, 0, len(models))
	for _, m := range models {
		modelIDs = append(modelIDs, m.ModelId)
	}
	h.pool.SetModelList(id, modelIDs)
	h.modelsCacheMu.Lock()
	h.cachedModels = mergeUniqueModels(h.cachedModels, models)
	h.modelsCacheTime = time.Now().Unix()
	h.modelsCacheMu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"models":  models,
	})
}

// apiGetAccountModelsCached 返回账号已缓存的模型列表（不实时拉取）
func (h *Handler) apiGetAccountModelsCached(w http.ResponseWriter, r *http.Request, id string) {
	models := h.pool.GetModelList(id)
	// Stable, readable order: subscription-covered entries (cline-pass/…)
	// first, then alphabetical.
	sort.SliceStable(models, func(i, j int) bool {
		pi, pj := strings.HasPrefix(models[i], "cline-pass/"), strings.HasPrefix(models[j], "cline-pass/")
		if pi != pj {
			return pi
		}
		return models[i] < models[j]
	})
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"models":  models,
	})
}

// ==================== 静态文件服务 ====================

func (h *Handler) serveAdminPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, "web/index.html")
}

// serveStaticFile serves the built dashboard from ./web. Hashed Vite assets
// under /admin/assets/ are immutable; any other unknown path falls back to
// index.html so client-side routes (/admin/accounts, ...) load on refresh.
func (h *Handler) serveStaticFile(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/admin/")
	if strings.Contains(rel, "..") {
		http.Error(w, "Not Found", 404)
		return
	}
	fsPath := filepath.Join("web", filepath.FromSlash(rel))
	if info, err := os.Stat(fsPath); err != nil || info.IsDir() {
		h.serveAdminPage(w, r)
		return
	}
	if strings.HasPrefix(rel, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeFile(w, r, fsPath)
}

// apiGetThinkingConfig 获取 thinking 配置
func (h *Handler) apiGetThinkingConfig(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetThinkingConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"suffix":       cfg.Suffix,
		"openaiFormat": cfg.OpenAIFormat,
		"claudeFormat": cfg.ClaudeFormat,
	})
}

// apiUpdateThinkingConfig 更新 thinking 配置
func (h *Handler) apiUpdateThinkingConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Suffix       string `json:"suffix"`
		OpenAIFormat string `json:"openaiFormat"`
		ClaudeFormat string `json:"claudeFormat"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// 验证格式
	validFormats := map[string]bool{"reasoning_content": true, "thinking": true, "think": true}
	if req.OpenAIFormat != "" && !validFormats[req.OpenAIFormat] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid openaiFormat, must be: reasoning_content, thinking, or think"})
		return
	}
	if req.ClaudeFormat != "" && !validFormats[req.ClaudeFormat] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid claudeFormat, must be: reasoning_content, thinking, or think"})
		return
	}

	if err := config.UpdateThinkingConfig(req.Suffix, req.OpenAIFormat, req.ClaudeFormat); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// apiGetEndpointConfig 获取端点配置
func (h *Handler) apiGetEndpointConfig(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"preferredEndpoint": config.GetPreferredEndpoint(),
		"endpointFallback":  config.GetEndpointFallback(),
	})
}

// apiUpdateEndpointConfig 更新端点配置
func (h *Handler) apiUpdateEndpointConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PreferredEndpoint string `json:"preferredEndpoint"`
		EndpointFallback  *bool  `json:"endpointFallback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	valid := map[string]bool{"auto": true, "kiro": true, "codewhisperer": true, "amazonq": true}
	if !valid[req.PreferredEndpoint] {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid endpoint, must be: auto, kiro, codewhisperer, or amazonq"})
		return
	}

	if err := config.UpdatePreferredEndpoint(req.PreferredEndpoint); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if req.EndpointFallback != nil {
		config.UpdateEndpointFallback(*req.EndpointFallback)
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// applyProxyConfig 将代理配置应用到所有出站 HTTP 客户端（Kiro API + auth 模块）
func applyProxyConfig(proxyURL string) {
	initHTTPClients(proxyURL)
	auth.InitHttpClient(proxyURL)
}

// apiGetProxy returns the outbound mode: the socks5/http proxy URL and whether
// the egress relay is the selected mode. The two are mutually exclusive.
func (h *Handler) apiGetProxy(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"proxyURL":  config.GetProxyURL(),
		"useRelay":  config.IsRelayEnabled(),
		"proxyPool": config.GetProxyPool(),
	})
}

// apiUpdateProxy sets the outbound egress mode. useRelay=true selects the egress
// relay (and clears any socks5/http proxy); otherwise the proxyURL (possibly
// empty = Direct) is used and the relay is deselected. Relay and proxy are
// mutually exclusive outbound modes.
func (h *Handler) apiUpdateProxy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProxyURL  string    `json:"proxyURL"`
		UseRelay  bool      `json:"useRelay"`
		ProxyPool *[]string `json:"proxyPool,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	if req.ProxyPool != nil {
		pool := make([]string, 0, len(*req.ProxyPool))
		for _, u := range *req.ProxyPool {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			if !isSupportedProxyURL(u) {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "proxy pool entry must start with http://, https://, socks5://, or socks5h://: " + u})
				return
			}
			pool = append(pool, u)
		}
		if err := config.UpdateProxyPool(pool); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}

	if req.UseRelay {
		// Selecting the relay: require a configured relay URL, enable it (which
		// clears the proxy URL), and reset the HTTP clients to direct dialing — the
		// relay RoundTripper wraps every client and takes over once active.
		if relayURL, _ := config.GetRelaySettings(); strings.TrimSpace(relayURL) == "" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "configure a Relay URL in the Egress Relay section first"})
			return
		}
		if err := config.SetRelayEnabled(true); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		applyProxyConfig("")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	// Non-relay mode: validate the proxy URL (empty = Direct).
	if req.ProxyURL != "" {
		if !isSupportedProxyURL(req.ProxyURL) {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "proxyURL must start with http://, https://, socks5://, or socks5h://"})
			return
		}
	}
	if err := config.SetRelayEnabled(false); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if err := config.UpdateProxySettings(req.ProxyURL); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	applyProxyConfig(req.ProxyURL)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func isSupportedProxyURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") ||
		strings.HasPrefix(u, "socks5://") || strings.HasPrefix(u, "socks5h://")
}

// apiGetRelay returns the egress relay config. The secret is not echoed back in
// full — only whether one is set — so it is not re-exposed to the admin client.
func (h *Handler) apiGetRelay(w http.ResponseWriter, r *http.Request) {
	relayURL, secret := config.GetRelaySettings()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"relayUrl":  relayURL,
		"hasSecret": secret != "",
	})
}

// apiUpdateRelay sets the egress relay URL + shared secret. An empty relayUrl
// disables the relay (direct / ProxyURL egress resumes). A blank secret in the
// request keeps the existing one, so the UI can update the URL without re-typing
// the secret.
func (h *Handler) apiUpdateRelay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RelayURL    string `json:"relayUrl"`
		RelaySecret string `json:"relaySecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	req.RelayURL = strings.TrimSpace(req.RelayURL)
	if req.RelayURL != "" && !strings.HasPrefix(req.RelayURL, "http://") && !strings.HasPrefix(req.RelayURL, "https://") {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "relayUrl must start with http:// or https://"})
		return
	}
	secret := req.RelaySecret
	if secret == "" {
		// Preserve the existing secret when the field is left blank.
		_, secret = config.GetRelaySettings()
	}
	if req.RelayURL == "" {
		secret = "" // clearing the relay clears its secret too
	}
	if err := config.UpdateRelaySettings(req.RelayURL, secret); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// apiTestRelay sends a probe request THROUGH the given (or saved) relay to a
// harmless upstream and reports what came back, so the operator can verify the
// relay is reachable, the secret matches, and forwarding works before relying on
// it. A blank secret in the request falls back to the saved one.
func (h *Handler) apiTestRelay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RelayURL    string `json:"relayUrl"`
		RelaySecret string `json:"relaySecret"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	relayURL := strings.TrimSpace(req.RelayURL)
	secret := req.RelaySecret
	savedURL, savedSecret := config.GetRelaySettings()
	if relayURL == "" {
		relayURL, secret = savedURL, savedSecret
	} else if secret == "" {
		secret = savedSecret
	}
	if relayURL == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "no relay configured to test"})
		return
	}

	// Probe the relay with the X-Relay-Ping header: a ping-aware relay validates
	// the secret and returns 200 "relay-ok" WITHOUT forwarding, giving a definitive
	// positive that doesn't depend on what any upstream returns. Older relays
	// (deployed before ping) ignore the header and forward the request instead —
	// handled by the classification below. The probe target is a real allow-listed
	// AWS host so a forwarding relay still gets a valid HTTP response.
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: egress.NewRelayTransportWith(http.DefaultTransport, relayURL, secret),
	}
	probeReq, _ := http.NewRequest("GET", "https://oidc.us-east-1.amazonaws.com/", nil)
	probeReq.Header.Set("X-Relay-Ping", "1")
	resp, err := client.Do(probeReq)
	if err != nil {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "relay unreachable: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	bodyStr := strings.ToLower(strings.TrimSpace(string(body)))

	// Classify. Only a RELAY-level rejection is a failure; an upstream status
	// (e.g. AWS answering 403 to a bare GET) means the relay forwarded fine.
	var ok bool
	var reason string
	switch {
	case resp.StatusCode == 200 && strings.Contains(bodyStr, "relay-ok"):
		ok, reason = true, "ping ok" // ping-aware relay, secret verified
	case resp.StatusCode == 401 && (bodyStr == "" || strings.Contains(bodyStr, "unauthor")):
		ok, reason = false, "wrong secret (relay returned 401 unauthorized)"
	case resp.StatusCode == 403 && strings.Contains(bodyStr, "target not allowed"):
		ok, reason = false, "target host not on the relay allow-list"
	default:
		// Any other HTTP response came back THROUGH the relay from upstream.
		ok, reason = true, "forwarded to upstream"
	}
	out := map[string]interface{}{
		"ok":     ok,
		"status": resp.StatusCode,
		"detail": reason,
	}
	if !ok {
		out["error"] = reason
	}
	json.NewEncoder(w).Encode(out)
}

// apiGetRelaySource returns the embedded relay source for a platform with the
// shared secret BAKED IN, so the operator only deploys the code — no need to set
// a RELAY_KEY environment variable on the serverless platform. If no relay secret
// exists yet, one is generated and persisted (keeping any URL already entered) so
// the baked code and the app agree on the secret.
func (h *Handler) apiGetRelaySource(w http.ResponseWriter, r *http.Request) {
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	src, ok := relay.SourceFor(platform)
	if !ok {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "unknown platform (want cloudflare|vercel|deno)"})
		return
	}

	relayURL, secret := config.GetRelaySettings()
	if secret == "" {
		secret = generateRelaySecret()
		if err := config.UpdateRelaySettings(relayURL, secret); err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}
	// secret is URL-safe base64 (no quotes/backslashes), so it drops safely into
	// the JS/TS string literal placeholder.
	src.Code = strings.ReplaceAll(src.Code, "__RELAY_KEY__", secret)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"platform": src.Platform,
		"filename": src.Filename,
		"language": src.Language,
		"code":     src.Code,
		"secret":   secret,
	})
}

// generateRelaySecret returns a 256-bit URL-safe random token for the relay
// shared secret.
func generateRelaySecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "relay-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// apiGetVersion 获取版本信息
func (h *Handler) apiGetVersion(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"version": config.Version,
	})
}

// apiExportAccounts 导出账号凭证
func (h *Handler) apiExportAccounts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"` // 为空则导出全部
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 如果 body 为空或解析失败，导出全部
		req.IDs = nil
	}

	accounts := config.GetAccounts()

	// 如果指定了 ID，只导出指定的
	if len(req.IDs) > 0 {
		idSet := make(map[string]bool)
		for _, id := range req.IDs {
			idSet[id] = true
		}
		var filtered []config.Account
		for _, a := range accounts {
			if idSet[a.ID] {
				filtered = append(filtered, a)
			}
		}
		accounts = filtered
	}

	// 构建兼容 Kiro Account Manager 的导出格式
	type ExportCredentials struct {
		AccessToken  string `json:"accessToken"`
		CsrfToken    string `json:"csrfToken"`
		RefreshToken string `json:"refreshToken"`
		ClientID     string `json:"clientId,omitempty"`
		ClientSecret string `json:"clientSecret,omitempty"`
		Region       string `json:"region,omitempty"`
		ExpiresAt    int64  `json:"expiresAt"`
		AuthMethod   string `json:"authMethod,omitempty"`
		Provider     string `json:"provider,omitempty"`
	}

	type ExportSubscription struct {
		Type  string `json:"type"`
		Title string `json:"title,omitempty"`
	}

	type ExportUsage struct {
		Current     float64 `json:"current"`
		Limit       float64 `json:"limit"`
		PercentUsed float64 `json:"percentUsed"`
		LastUpdated int64   `json:"lastUpdated"`
	}

	type ExportAccount struct {
		ID           string             `json:"id"`
		Email        string             `json:"email"`
		Nickname     string             `json:"nickname,omitempty"`
		Idp          string             `json:"idp"`
		UserId       string             `json:"userId,omitempty"`
		MachineId    string             `json:"machineId,omitempty"`
		Credentials  ExportCredentials  `json:"credentials"`
		Subscription ExportSubscription `json:"subscription"`
		Usage        ExportUsage        `json:"usage"`
		Tags         []string           `json:"tags"`
		Status       string             `json:"status"`
		CreatedAt    int64              `json:"createdAt"`
		LastUsedAt   int64              `json:"lastUsedAt"`
	}

	type ExportData struct {
		Version    string          `json:"version"`
		ExportedAt int64           `json:"exportedAt"`
		Accounts   []ExportAccount `json:"accounts"`
		Groups     []interface{}   `json:"groups"`
		Tags       []interface{}   `json:"tags"`
	}

	exportAccounts := make([]ExportAccount, 0, len(accounts))
	for _, a := range accounts {
		// 映射 provider 到 idp
		idp := a.Provider
		if idp == "" {
			if a.AuthMethod == "social" {
				idp = "Google"
			} else {
				idp = "BuilderId"
			}
		}

		// 映射 authMethod
		authMethod := a.AuthMethod
		if authMethod == "idc" {
			authMethod = "IdC"
		}

		// 映射订阅类型
		subType := "Free"
		rawType := strings.ToUpper(a.SubscriptionType)
		if strings.Contains(rawType, "PRO_PLUS") || strings.Contains(rawType, "PROPLUS") {
			subType = "Pro_Plus"
		} else if strings.Contains(rawType, "PRO") {
			subType = "Pro"
		} else if strings.Contains(rawType, "POWER") {
			subType = "Pro_Plus"
		}

		exportAccounts = append(exportAccounts, ExportAccount{
			ID:        a.ID,
			Email:     a.Email,
			Nickname:  a.Nickname,
			Idp:       idp,
			UserId:    a.UserId,
			MachineId: a.MachineId,
			Credentials: ExportCredentials{
				AccessToken:  a.AccessToken,
				CsrfToken:    "",
				RefreshToken: a.RefreshToken,
				ClientID:     a.ClientID,
				ClientSecret: a.ClientSecret,
				Region:       a.Region,
				ExpiresAt:    a.ExpiresAt * 1000, // 转为毫秒时间戳
				AuthMethod:   authMethod,
				Provider:     a.Provider,
			},
			Subscription: ExportSubscription{
				Type:  subType,
				Title: a.SubscriptionTitle,
			},
			Usage: ExportUsage{
				Current:     a.UsageCurrent,
				Limit:       a.UsageLimit,
				PercentUsed: a.UsagePercent,
				LastUpdated: time.Now().UnixMilli(),
			},
			Tags:       []string{},
			Status:     "active",
			CreatedAt:  time.Now().UnixMilli(),
			LastUsedAt: time.Now().UnixMilli(),
		})
	}

	data := ExportData{
		Version:    config.Version,
		ExportedAt: time.Now().UnixMilli(),
		Accounts:   exportAccounts,
		Groups:     []interface{}{},
		Tags:       []interface{}{},
	}

	json.NewEncoder(w).Encode(data)
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
