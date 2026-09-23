package proxy

import (
	"encoding/json"
	"errors"
	"hekato-go/auth"
	"hekato-go/config"
	"hekato-go/logger"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Warmup keeps the pool healthy between requests: every cycle each account's
// token is refreshed, its quota/credits re-fetched, optionally a tiny chat
// probe is sent, and accounts that failover auto-disabled are re-enabled once
// they pass again. Runs on the account refresh interval and on demand.
// ponytail: fixed concurrency of 5 and two retries; expose as settings if needed.

const (
	warmupConcurrency = 5
	warmupRetries     = 2
)

type warmupResult struct {
	AccountID string `json:"accountId"`
	Email     string `json:"email"`
	Status    string `json:"status"` // ok | error | skipped
	Error     string `json:"error,omitempty"`
	Recovered bool   `json:"recovered,omitempty"`
	Probed    bool   `json:"probed,omitempty"`
	LatencyMs int64  `json:"latencyMs,omitempty"`
}

type warmupState struct {
	mu        sync.Mutex
	running   bool
	lastRun   int64
	lastTook  int64
	nextRun   int64
	lastCount struct{ Total, OK, Errors, Recovered int }
	results   []warmupResult
}

// autoBanned reports whether failover (not an operator) disabled the account.
func autoBanned(a *config.Account) bool {
	return !a.Enabled && a.BanReason != ""
}

// warmupCandidates: enabled accounts plus auto-banned ones when recovery is on.
func warmupCandidates(ids []string, recover bool) []config.Account {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []config.Account
	for _, a := range config.GetAccounts() {
		if len(want) > 0 && !want[a.ID] {
			continue
		}
		if a.AccessToken == "" && a.RefreshToken == "" {
			continue
		}
		if a.Enabled || (recover && autoBanned(&a)) || len(want) > 0 {
			out = append(out, a)
		}
	}
	return out
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}
	var ue *UpstreamError
	if errors.As(err, &ue) {
		return ue.Status >= 500 || ue.Status == 0
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "connection") || strings.Contains(msg, "eof")
}

// probeModelFor picks the model for the Test button and the warmup probe:
// the account's own probe model, else the global default test model when the
// account serves it, else the cheapest-looking advertised model (haiku /
// flash / lite / mini / small), else the first advertised, else a Claude
// default. Never the first catalog entry blindly: that can be the priciest.
func probeModelFor(account *config.Account) string {
	if account != nil && strings.TrimSpace(account.ProbeModel) != "" {
		return strings.TrimSpace(account.ProbeModel)
	}
	models, err := ListAvailableModels(account)
	if preferred := config.GetTestModel(); preferred != "" {
		if err != nil || len(models) == 0 {
			return preferred
		}
		for _, m := range models {
			if strings.EqualFold(m.ModelId, preferred) {
				return preferred
			}
		}
	}
	if err == nil && len(models) > 0 {
		if adapter, aerr := adapterForAccount(account); aerr == nil && adapter.probeModel != nil {
			if m := adapter.probeModel(models); m != "" {
				return m
			}
		}
		return cheapestModel(models)
	}
	return "claude-sonnet-4.5"
}

// cheapestModel prefers small / fast tiers by name; falls back to the first.
func cheapestModel(models []ModelInfo) string {
	for _, hint := range []string{"haiku", "flash-lite", "lite", "flash", "mini", "small", "nano"} {
		for _, m := range models {
			id := strings.ToLower(m.ModelId)
			if strings.Contains(id, hint) && !strings.Contains(id, "thinking") {
				return m.ModelId
			}
		}
	}
	return models[0].ModelId
}

// warmupOne runs the full check for one account and persists the outcome.
func (h *Handler) warmupOne(account *config.Account, probe, recover bool) warmupResult {
	res := warmupResult{AccountID: account.ID, Email: account.Email}
	start := time.Now()
	wasBanned := autoBanned(account)

	step := func(name string, fn func() error) error {
		var err error
		for attempt := 0; attempt <= warmupRetries; attempt++ {
			if err = fn(); err == nil || !isTransient(err) {
				return err
			}
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
		return err
	}

	// 1) token refresh when expiring (skipped for credentials with no refresh flow).
	refreshable := !isCodeBuddyAccount(account) || auth.CodeBuddyRefreshable(account)
	if refreshable && account.ExpiresAt > 0 && time.Now().Unix() > account.ExpiresAt-tokenRefreshSkewSeconds {
		if err := step("token", func() error {
			newAccess, newRefresh, newExp, profileArn, err := auth.RefreshToken(account)
			if err != nil {
				return err
			}
			account.AccessToken = newAccess
			if newRefresh != "" {
				account.RefreshToken = newRefresh
			}
			account.ExpiresAt = newExp
			config.UpdateAccountToken(account.ID, newAccess, newRefresh, newExp)
			h.pool.UpdateToken(account.ID, newAccess, newRefresh, newExp)
			if profileArn != "" {
				account.ProfileArn = profileArn
				config.UpdateAccountProfileArn(account.ID, profileArn)
			}
			// Codex may mutate UserId (chatgpt_account_id) on refresh; persist it
			// so the next request's chatgpt-account-id header uses the fresh tenant.
			if isCodexAccount(account) && account.UserId != "" {
				config.UpdateAccountUserId(account.ID, account.UserId)
			}
			return nil
		}); err != nil {
			return h.finishWarmup(res, account, err, false)
		}
	}

	// 2) quota / credits.
	if err := step("usage", func() error {
		info, err := RefreshAccountInfo(account)
		if err != nil {
			return err
		}
		config.UpdateAccountInfo(account.ID, *info)
		return nil
	}); err != nil {
		return h.finishWarmup(res, account, err, false)
	}

	// 3) optional inference probe ("Say OK").
	if probe {
		res.Probed = true
		maxTokens := 4
		if isCodeBuddyAccount(account) {
			maxTokens = 0
		}
		req := &OpenAIRequest{Model: probeModelFor(account), MaxTokens: maxTokens, Stream: true,
			Messages: []OpenAIMessage{{Role: "user", Content: "Say OK"}}}
		if err := step("probe", func() error {
			return callUpstreamFromOpenAI(account, req, false, &StreamCallback{})
		}); err != nil {
			return h.finishWarmup(res, account, err, false)
		}
	}

	res.LatencyMs = time.Since(start).Milliseconds()
	if wasBanned && recover {
		if err := config.RecoverAccount(account.ID); err == nil {
			res.Recovered = true
			logger.Infof("[Warmup] Recovered %s (was: %s)", account.Email, account.BanReason)
		}
	}
	return h.finishWarmup(res, account, nil, wasBanned && recover)
}

func (h *Handler) finishWarmup(res warmupResult, account *config.Account, err error, recovered bool) warmupResult {
	now := time.Now().Unix()
	if err != nil {
		res.Status = "error"
		res.Error = err.Error()
		if len(res.Error) > 500 {
			res.Error = res.Error[:500]
		}
		config.UpdateAccountWarmup(account.ID, "error", res.Error, now)
		logger.Warnf("[Warmup] %s failed: %s", account.Email, res.Error)
		if account.Enabled {
			h.handleAccountFailure(account, err)
		} else {
			logger.Infof("[Warmup] %s is disabled; failure not fed to failover", account.Email)
		}
		return res
	}
	res.Status = "ok"
	config.UpdateAccountWarmup(account.ID, "ok", "", now)
	if account.Enabled || recovered {
		h.pool.RecordSuccess(account.ID)
	}
	return res
}

// runWarmup checks the given accounts (nil = all candidates) with bounded
// concurrency and returns per-account results. Only one run at a time.
func (h *Handler) runWarmup(ids []string) []warmupResult {
	h.warmup.mu.Lock()
	if h.warmup.running {
		h.warmup.mu.Unlock()
		return nil
	}
	h.warmup.running = true
	h.warmup.mu.Unlock()
	start := time.Now()

	probe, recover := config.GetWarmupOptions()
	cands := warmupCandidates(ids, recover)
	results := make([]warmupResult, len(cands))
	sem := make(chan struct{}, warmupConcurrency)
	var wg sync.WaitGroup
	for i := range cands {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			acc := cands[i]
			results[i] = h.warmupOne(&acc, probe, recover)
		}(i)
	}
	wg.Wait()
	h.pool.Reload()

	h.warmup.mu.Lock()
	h.warmup.running = false
	h.warmup.lastRun = start.Unix()
	h.warmup.lastTook = time.Since(start).Milliseconds()
	h.warmup.nextRun = time.Now().Add(accountRefreshInterval()).Unix()
	h.warmup.lastCount.Total, h.warmup.lastCount.OK, h.warmup.lastCount.Errors, h.warmup.lastCount.Recovered = 0, 0, 0, 0
	for _, r := range results {
		h.warmup.lastCount.Total++
		switch r.Status {
		case "ok":
			h.warmup.lastCount.OK++
		case "error":
			h.warmup.lastCount.Errors++
		}
		if r.Recovered {
			h.warmup.lastCount.Recovered++
		}
	}
	h.warmup.results = results
	h.warmup.mu.Unlock()
	logger.Infof("[Warmup] %d accounts checked: %d ok, %d errors, %d recovered (%dms)",
		len(results), h.warmup.lastCount.OK, h.warmup.lastCount.Errors, h.warmup.lastCount.Recovered, h.warmup.lastTook)
	return results
}

// ---- admin API ----

// apiWarmupStatus: GET /admin/api/warmup/status
func (h *Handler) apiWarmupStatus(w http.ResponseWriter, r *http.Request) {
	probe, recover := config.GetWarmupOptions()
	h.warmup.mu.Lock()
	defer h.warmup.mu.Unlock()
	results := h.warmup.results
	if results == nil {
		results = []warmupResult{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"running":         h.warmup.running,
		"lastRun":         h.warmup.lastRun,
		"lastTookMs":      h.warmup.lastTook,
		"nextRun":         h.warmup.nextRun,
		"intervalMinutes": int(accountRefreshInterval().Minutes()),
		"probe":           probe,
		"recover":         recover,
		"summary":         h.warmup.lastCount,
		"results":         results,
	})
}

// apiRunWarmup: POST /admin/api/warmup {"ids": [...]} (empty = all)
func (h *Handler) apiRunWarmup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	results := h.runWarmup(req.IDs)
	if results == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "warmup already running"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "results": results})
}
