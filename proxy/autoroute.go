package proxy

import (
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/pool"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// The virtual "auto" model. When auto routing is enabled, requests for it are
// classified into a tier and an (account, model) pair is picked by a bandit.
const autoModelName = "auto"

// routeSignals are the locally observable complexity signals of a request.
type routeSignals struct {
	InputTokens int  `json:"inputTokens"`
	Tools       int  `json:"tools"`
	Turns       int  `json:"turns"`
	Images      bool `json:"images"`
	Thinking    bool `json:"thinking"`
}

var tierNames = [3]string{"fast", "balanced", "strong"}

// classifyTier maps signals to a tier index (0 fast, 1 balanced, 2 strong).
// ponytail: fixed thresholds; make them configurable if operators ask.
func classifyTier(s routeSignals) int {
	switch {
	case s.Thinking || s.InputTokens > 40000 || s.Tools >= 8:
		return 2
	case s.InputTokens > 6000 || s.Tools > 0 || s.Turns > 6 || s.Images:
		return 1
	default:
		return 0
	}
}

// candidateStats is the decayed reliability + latency record of one (account, model).
type candidateStats struct {
	Successes   float64 `json:"successes"`
	Failures    float64 `json:"failures"`
	EwmaLatency float64 `json:"ewmaLatencyMs"`
	Updated     time.Time
}

const statsHalfLife = time.Hour

func (c *candidateStats) decay(now time.Time) {
	if c.Updated.IsZero() {
		c.Updated = now
		return
	}
	if dt := now.Sub(c.Updated); dt > 0 {
		f := math.Pow(0.5, dt.Hours()/statsHalfLife.Hours())
		c.Successes *= f
		c.Failures *= f
	}
	c.Updated = now
}

// routeDecision is what the router chose and why; kept in a ring for the dashboard.
type routeDecision struct {
	Time      int64        `json:"time"`
	Endpoint  string       `json:"endpoint"`
	Tier      string       `json:"tier"`
	Model     string       `json:"model"`
	AccountID string       `json:"accountId"`
	Score     float64      `json:"score"`
	Explored  bool         `json:"explored"`
	Pinned    bool         `json:"pinned"`
	Signals   routeSignals `json:"signals"`
	Reason    string       `json:"reason"`
}

const decisionsRingSize = 200

type autoRouter struct {
	mu        sync.Mutex
	stats     map[string]*candidateStats // "accountID|model"
	decisions []routeDecision
	rng       *rand.Rand
	store     config.BlobStore
	dirty     bool
}

const autoRouterBlobKey = "autoroute"

// autoRouterState is the persisted form: learned stats + recent decisions.
type autoRouterState struct {
	Stats     map[string]*candidateStats `json:"stats"`
	Decisions []routeDecision            `json:"decisions"`
}

// Load restores state from the blob store (no-op when nil / empty).
func (r *autoRouter) Load(store config.BlobStore) {
	if r == nil || store == nil {
		return
	}
	r.mu.Lock()
	r.store = store
	r.mu.Unlock()
	data, err := store.LoadBlob(autoRouterBlobKey)
	if err != nil {
		logger.Warnf("[AutoRoute] load failed: %v", err)
		return
	}
	if data == "" {
		return
	}
	var st autoRouterState
	if err := json.Unmarshal([]byte(data), &st); err != nil {
		logger.Warnf("[AutoRoute] load: bad state: %v", err)
		return
	}
	r.mu.Lock()
	if st.Stats != nil {
		r.stats = st.Stats
	}
	r.decisions = st.Decisions
	r.mu.Unlock()
	logger.Infof("[AutoRoute] restored %d candidates, %d decisions", len(st.Stats), len(st.Decisions))
}

// Flush persists state when it changed since the last flush.
func (r *autoRouter) Flush() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.store == nil || !r.dirty {
		r.mu.Unlock()
		return
	}
	data, err := json.Marshal(autoRouterState{Stats: r.stats, Decisions: r.decisions})
	r.dirty = false
	store := r.store
	r.mu.Unlock()
	if err != nil {
		return
	}
	if err := store.SaveBlob(autoRouterBlobKey, string(data)); err != nil {
		logger.Warnf("[AutoRoute] save failed: %v", err)
		r.mu.Lock()
		r.dirty = true
		r.mu.Unlock()
	}
}

func newAutoRouter() *autoRouter {
	return &autoRouter{stats: make(map[string]*candidateStats), rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func statKey(accountID, model string) string { return accountID + "|" + strings.ToLower(model) }

func isAutoModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), autoModelName)
}

func matchesTier(model string, patterns []string) bool {
	m := strings.ToLower(model)
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" && strings.Contains(m, p) {
			return true
		}
	}
	return false
}

// thompsonSample approximates a Beta(s+1, f+1) draw with a clamped normal.
// ponytail: normal approximation; swap for a real gamma sampler if the bandit ever needs exactness.
func (r *autoRouter) thompsonSample(s, f float64) float64 {
	a, b := s+1, f+1
	mean := a / (a + b)
	variance := (a * b) / ((a + b) * (a + b) * (a + b + 1))
	x := mean + r.rng.NormFloat64()*math.Sqrt(variance)
	return math.Max(0.01, math.Min(1, x))
}

type routeCandidate struct {
	account *config.Account
	model   string
}

// Resolve picks a concrete (account, model) for an "auto" request. It returns
// nil when nothing in the pool matches any tier (caller falls back to normal
// routing with the balanced default).
func (r *autoRouter) Resolve(p *pool.AccountPool, cfg config.AutoRouteConfig, sig routeSignals, filter pool.AccountFilter, endpoint string) *routeDecision {
	if r == nil || p == nil {
		return nil
	}
	tier := classifyTier(sig)
	// Quality pushes up, cost pushes down. Only a clear preference moves the tier.
	shift := cfg.QualityWeight - cfg.CostWeight
	if shift >= 0.5 {
		tier++
	} else if shift <= -0.5 {
		tier--
	}
	tier = int(math.Max(0, math.Min(2, float64(tier))))
	tiers := [3][]string{cfg.Fast, cfg.Balanced, cfg.Strong}

	// Walk from the chosen tier outward until some account offers a model.
	order := []int{tier}
	for d := 1; d <= 2; d++ {
		if tier+d <= 2 {
			order = append(order, tier+d)
		}
		if tier-d >= 0 {
			order = append(order, tier-d)
		}
	}
	var cands []routeCandidate
	usedTier := tier
	for _, ti := range order {
		cands = r.candidates(p, tiers[ti], filter, cfg)
		if len(cands) > 0 {
			usedTier = ti
			break
		}
	}
	if len(cands) == 0 {
		return nil
	}

	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	explored := r.rng.Float64() < cfg.Explore
	best := -1.0
	var pick routeCandidate
	var pickScore float64
	if explored {
		pick = cands[r.rng.Intn(len(cands))]
	}
	for _, c := range cands {
		st := r.stats[statKey(c.account.ID, c.model)]
		if st == nil {
			st = &candidateStats{}
		}
		st.decay(now)
		reliability := r.thompsonSample(st.Successes, st.Failures)
		speed := 1.0
		if st.EwmaLatency > 0 {
			speed = 1 / (1 + st.EwmaLatency/2000)
		}
		budget := 1.0
		if c.account.UsageLimit > 0 {
			budget = math.Max(0, 1-c.account.UsagePercent)
		}
		score := reliability * (1 - cfg.SpeedWeight*(1-speed)) * (1 - cfg.CostWeight*(1-budget))
		if explored {
			if c == pick {
				pickScore = score
			}
			continue
		}
		if score > best {
			best, pick, pickScore = score, c, score
		}
	}

	reason := fmt.Sprintf("tier=%s tokens=%d tools=%d turns=%d thinking=%t images=%t candidates=%d",
		tierNames[usedTier], sig.InputTokens, sig.Tools, sig.Turns, sig.Thinking, sig.Images, len(cands))
	if usedTier != tier {
		reason += fmt.Sprintf(" (wanted %s)", tierNames[tier])
	}
	if explored {
		reason += " explore"
	}
	d := routeDecision{
		Time: now.Unix(), Endpoint: endpoint, Tier: tierNames[usedTier], Model: pick.model,
		AccountID: pick.account.ID, Score: math.Round(pickScore*1000) / 1000, Explored: explored,
		Signals: sig, Reason: reason,
	}
	r.pushLocked(d)
	return &d
}

// candidates lists routable (account, model) pairs whose model matches the tier.
func (r *autoRouter) candidates(p *pool.AccountPool, patterns []string, filter pool.AccountFilter, cfg config.AutoRouteConfig) []routeCandidate {
	if len(patterns) == 0 {
		return nil
	}
	var out []routeCandidate
	seen := make(map[string]bool)
	for _, acc := range p.GetAllAccounts() {
		if seen[acc.ID] {
			continue
		}
		seen[acc.ID] = true
		models := p.GetModelList(acc.ID)
		prov, _ := config.ProviderForAccount(&acc)
		for _, m := range models {
			if !matchesTier(m, patterns) || cfg.Blacklisted(prov, m) {
				continue
			}
			if a := p.GetForModelByID(acc.ID, m, filter); a != nil {
				out = append(out, routeCandidate{account: a, model: m})
			}
		}
	}
	return out
}

// Record feeds an outcome back into the bandit.
func (r *autoRouter) Record(accountID, model string, success bool, latencyMs int64) {
	if r == nil || accountID == "" || model == "" {
		return
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	k := statKey(accountID, model)
	st := r.stats[k]
	if st == nil {
		st = &candidateStats{}
		r.stats[k] = st
	}
	st.decay(now)
	r.dirty = true
	if success {
		st.Successes++
		if latencyMs > 0 {
			if st.EwmaLatency == 0 {
				st.EwmaLatency = float64(latencyMs)
			} else {
				st.EwmaLatency = 0.8*st.EwmaLatency + 0.2*float64(latencyMs)
			}
		}
	} else {
		st.Failures++
	}
}

func (r *autoRouter) RecordPinned(d routeDecision) {
	r.mu.Lock()
	r.pushLocked(d)
	r.mu.Unlock()
}

func (r *autoRouter) pushLocked(d routeDecision) {
	if len(r.decisions) >= decisionsRingSize {
		r.decisions = r.decisions[1:]
	}
	r.decisions = append(r.decisions, d)
	r.dirty = true
}

type candidateView struct {
	AccountID   string  `json:"accountId"`
	Provider    string  `json:"provider"`
	Email       string  `json:"email"`
	Model       string  `json:"model"`
	Successes   float64 `json:"successes"`
	Failures    float64 `json:"failures"`
	EwmaLatency float64 `json:"ewmaLatencyMs"`
	Reliability float64 `json:"reliability"`
	LastUpdated int64   `json:"lastUpdated"`
}

// Snapshot returns recent decisions (newest first) and per-candidate stats.
func (r *autoRouter) Snapshot() ([]routeDecision, []candidateView) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	dec := make([]routeDecision, len(r.decisions))
	for i, d := range r.decisions {
		dec[len(r.decisions)-1-i] = d
	}
	providers, emails := map[string]string{}, map[string]string{}
	for _, a := range config.GetAccounts() {
		emails[a.ID] = a.Email
		if prov, err := config.ProviderForAccount(&a); err == nil {
			providers[a.ID] = string(prov)
		}
	}
	var cands []candidateView
	for k, st := range r.stats {
		st.decay(now)
		parts := strings.SplitN(k, "|", 2)
		cands = append(cands, candidateView{
			AccountID: parts[0], Provider: providers[parts[0]], Email: emails[parts[0]], Model: parts[1],
			Successes: math.Round(st.Successes*100) / 100, Failures: math.Round(st.Failures*100) / 100,
			EwmaLatency: math.Round(st.EwmaLatency),
			Reliability: math.Round((st.Successes+1)/(st.Successes+st.Failures+2)*1000) / 1000,
			LastUpdated: st.Updated.Unix(),
		})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Reliability > cands[j].Reliability })
	return dec, cands
}

// resolveAutoModel applies the router to a request for the virtual "auto"
// model. It returns the concrete model to use and pins the chosen account on
// the affinity key (creating a per-request key if the conversation has none)
// so the normal pickAccount path selects it. Headers announce the decision.
func (h *Handler) resolveAutoModel(w http.ResponseWriter, endpoint, model string, sig routeSignals, affinityKey *string, cap providerCapability) string {
	if !isAutoModel(model) {
		return model
	}
	cfg := config.GetAutoRouteConfig()
	if !cfg.Enabled {
		return model
	}
	// Cache affinity wins: a pinned conversation keeps its account and model.
	if *affinityKey != "" {
		if accID, pinnedModel := h.affinity.GetWithModel(*affinityKey); accID != "" && pinnedModel != "" && !isAutoModel(pinnedModel) {
			if h.pool.GetForModelByID(accID, pinnedModel, capabilityFilter(cap)) != nil {
				h.autoRouter.RecordPinned(routeDecision{Time: time.Now().Unix(), Endpoint: endpoint, Tier: "pinned", Model: pinnedModel, AccountID: accID, Pinned: true, Signals: sig, Reason: "conversation pinned (warm cache)"})
				w.Header().Set("X-Hekato-Routed-Model", pinnedModel)
				w.Header().Set("X-Hekato-Route-Reason", "pinned")
				return pinnedModel
			}
		}
	}
	d := h.autoRouter.Resolve(h.pool, cfg, sig, capabilityFilter(cap), endpoint)
	if d == nil {
		// No tier has a candidate. Fall back to a concrete model that a routable
		// account actually advertises (balanced, then fast, then strong patterns,
		// then anything) so the literal "auto" never leaks upstream while
		// routing is enabled. Only when no account advertises any model at all
		// is "auto" passed through (Kiro / CodeBuddy CN serve it natively).
		if fb := h.fallbackAutoModel(cfg, capabilityFilter(cap)); fb != "" {
			w.Header().Set("X-Hekato-Routed-Model", fb)
			w.Header().Set("X-Hekato-Route-Reason", "no tier candidates; fallback to advertised model")
			h.autoRouter.RecordPinned(routeDecision{Time: time.Now().Unix(), Endpoint: endpoint, Tier: "fallback", Model: fb, Signals: sig, Reason: "no tier candidates; first advertised model"})
			return fb
		}
		logger.Warnf("[AutoRoute] no routable model advertised for an auto request; passing \"auto\" upstream")
		return model
	}
	if *affinityKey == "" {
		// Per-request key so pickAccount honours the router's choice; uuid is
		// goroutine-safe (the router's rand.Rand is only used under its mutex).
		*affinityKey = "auto:" + uuid.NewString()
	}
	h.affinity.SetWithModel(*affinityKey, d.AccountID, d.Model)
	w.Header().Set("X-Hekato-Routed-Model", d.Model)
	w.Header().Set("X-Hekato-Route-Reason", d.Reason)
	return d.Model
}

// fallbackAutoModel picks a concrete model when no tier has candidates.
func (h *Handler) fallbackAutoModel(cfg config.AutoRouteConfig, filter pool.AccountFilter) string {
	seen := map[string]bool{}
	var advertised []string
	for _, acc := range h.pool.GetAllAccounts() {
		if seen[acc.ID] {
			continue
		}
		seen[acc.ID] = true
		for _, m := range h.pool.GetModelList(acc.ID) {
			if isAutoModel(m) {
				continue
			}
			if h.pool.GetForModelByID(acc.ID, m, filter) != nil {
				advertised = append(advertised, m)
			}
		}
	}
	for _, patterns := range [][]string{cfg.Balanced, cfg.Fast, cfg.Strong} {
		for _, m := range advertised {
			if matchesTier(m, patterns) {
				return m
			}
		}
	}
	if len(advertised) > 0 {
		return advertised[0]
	}
	return ""
}

func claudeRouteSignals(req *ClaudeRequest, inputTokens int, thinking bool) routeSignals {
	s := routeSignals{InputTokens: inputTokens, Tools: len(req.Tools), Turns: len(req.Messages), Thinking: thinking}
	for _, m := range req.Messages {
		if blocks, ok := m.Content.([]interface{}); ok {
			for _, b := range blocks {
				if bm, ok := b.(map[string]interface{}); ok && bm["type"] == "image" {
					s.Images = true
				}
			}
		}
	}
	return s
}

func openAIRouteSignals(req *OpenAIRequest, inputTokens int, thinking bool) routeSignals {
	s := routeSignals{InputTokens: inputTokens, Tools: len(req.Tools), Turns: len(req.Messages), Thinking: thinking}
	for _, m := range req.Messages {
		if parts, ok := m.Content.([]interface{}); ok {
			for _, p := range parts {
				if pm, ok := p.(map[string]interface{}); ok && (pm["type"] == "image_url" || pm["type"] == "input_image") {
					s.Images = true
				}
			}
		}
	}
	return s
}

// ---- admin API ----

func (h *Handler) apiGetAutoRoute(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(config.GetAutoRouteConfig())
}

func (h *Handler) apiUpdateAutoRoute(w http.ResponseWriter, r *http.Request) {
	var req config.AutoRouteConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}
	clamp := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
	req.QualityWeight, req.CostWeight, req.SpeedWeight = clamp(req.QualityWeight), clamp(req.CostWeight), clamp(req.SpeedWeight)
	req.Explore = math.Max(0, math.Min(0.5, req.Explore))
	if len(req.Fast)+len(req.Balanced)+len(req.Strong) == 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "at least one tier needs a model pattern"})
		return
	}
	if err := config.UpdateAutoRouteConfig(req); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) apiGetAutoRouteDecisions(w http.ResponseWriter, r *http.Request) {
	dec, cands := h.autoRouter.Snapshot()
	if dec == nil {
		dec = []routeDecision{}
	}
	if cands == nil {
		cands = []candidateView{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"decisions": dec, "candidates": cands})
}
