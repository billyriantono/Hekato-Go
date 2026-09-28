package proxy

import (
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/pool"
	"hekato-go/providers/modelsdev"
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
// Content-derived flags are booleans by design: signals are persisted in the
// decision ring and shown in the admin panel, so the prompt text that produced
// them is deliberately not carried here.
type routeSignals struct {
	InputTokens int  `json:"inputTokens"`
	Tools       int  `json:"tools"`
	Turns       int  `json:"turns"`
	Images      bool `json:"images"`
	Thinking    bool `json:"thinking"`
	// Code, Reasoning and Simple describe the last user message.
	Code      bool `json:"code,omitempty"`
	Reasoning bool `json:"reasoning,omitempty"`
	Simple    bool `json:"simple,omitempty"`
	// KeywordTier is a tier forced by an operator rule (0 = unset, else tier+1,
	// so the zero value of the struct means "no rule matched").
	KeywordTier int `json:"keywordTier,omitempty"`
}

var tierNames = [3]string{"fast", "balanced", "strong"}

// Tier thresholds over the additive complexity score below.
// ponytail: fixed weights; make them configurable if operators ask.
const (
	strongScore   = 4
	balancedScore = 2
)

// classifyTier scores a request and maps the score to a tier (0 fast,
// 1 balanced, 2 strong).
//
// This used to be a cascade in which "tools >= 8" meant strong. Every coding
// agent sends its whole toolset on every turn, so in production that single
// clause decided nearly everything: 19 of 25 classified requests arrived with
// exactly 13 tools, and a 300-token "hi" was sorted with a 70k-token refactor.
// Tool count is now worth one point among several, and what the user actually
// wrote carries real weight.
func classifyTier(s routeSignals) int {
	// An operator keyword rule is an instruction, not a hint.
	if s.KeywordTier > 0 {
		return s.KeywordTier - 1
	}
	score := 0
	switch {
	// A prompt this large is heavy work on its own terms, whatever else the
	// request looks like — decisive, like explicit thinking, but still
	// cancellable by a one-word message.
	case s.InputTokens > 40000:
		score += strongScore
	case s.InputTokens > 12000:
		score += 2
	case s.InputTokens > 4000:
		score++
	}
	if s.Turns > 20 {
		score++
	}
	if s.Tools > 0 {
		score++
	}
	if s.Images {
		score++
	}
	if s.Code {
		score++
	}
	if s.Reasoning {
		score += 2
	}
	// A client explicitly asking for extended thinking is stating intent, not
	// leaving a hint to be weighed: on its own it reaches the strong tier. It
	// can still be pulled back down by a trivial message ("ok" with thinking
	// on is not hard work).
	if s.Thinking {
		score += strongScore
	}
	// A one-line acknowledgement is cheap work whatever else the envelope says.
	if s.Simple {
		score -= 2
	}
	switch {
	case score >= strongScore:
		return 2
	case score >= balancedScore:
		return 1
	default:
		return 0
	}
}

// effectiveTier is the tier a request routes to: the classified tier moved by
// the quality/cost preference. Resolve and the affinity-pin check must agree on
// it, or a pinned conversation is judged against a tier it never wanted.
func effectiveTier(cfg config.AutoRouteConfig, sig routeSignals) int {
	tier := classifyTier(sig)
	// Quality pushes up, cost pushes down. Only a clear preference moves the tier.
	shift := cfg.QualityWeight - cfg.CostWeight
	if shift >= 0.5 {
		tier++
	} else if shift <= -0.5 {
		tier--
	}
	return int(math.Max(0, math.Min(2, float64(tier))))
}

// tierOfModel reports the highest tier whose patterns match model, or -1 when
// no tier claims it. Highest wins because patterns overlap (glm-5.3-flash sits
// in both fast and balanced) and the generous reading avoids evicting a pin
// that is in fact strong enough.
func tierOfModel(cfg config.AutoRouteConfig, model string) int {
	found := -1
	for i, patterns := range [3][]string{cfg.Fast, cfg.Balanced, cfg.Strong} {
		if matchesTier(model, patterns) {
			found = i
		}
	}
	return found
}

// candidateStats is the decayed reliability + latency record of one (account, model).
type candidateStats struct {
	Successes   float64 `json:"successes"`
	Failures    float64 `json:"failures"`
	EwmaLatency float64 `json:"ewmaLatencyMs"`
	Updated     time.Time
	// QuarantinedUntil sidelines a pair the upstream keeps rejecting outright.
	// Decayed reliability alone cannot express this: a hard rejection is
	// certainty, not evidence to be averaged away.
	QuarantinedUntil time.Time `json:"quarantinedUntil,omitempty"`
	QuarantineReason string    `json:"quarantineReason,omitempty"`
}

func (c *candidateStats) quarantined(now time.Time) bool {
	return c != nil && now.Before(c.QuarantinedUntil)
}

// statsHalfLife governs how fast the bandit forgets. An hour sounded prudent
// but decayed faster than traffic accumulated per (account, model): every
// candidate sat back at Beta(1,1) — a coin flip dressed up as Thompson
// sampling. A day keeps enough signal to actually rank candidates.
const statsHalfLife = 24 * time.Hour

// quarantineWindow is how long a candidate sits out after a hard failure —
// one the upstream will repeat for the same (account, model) pair, e.g. "model
// not supported". Long enough to outlast a model-list refresh, short enough
// that a fixed upstream returns on its own.
const quarantineWindow = 30 * time.Minute

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
	Time     int64  `json:"time"`
	Endpoint string `json:"endpoint"`
	Tier     string `json:"tier"`
	// WantedTier is the tier the request classified into, when it differs from
	// the tier that actually served it. A populated value means that tier was
	// starved — no candidate offered it — which is invisible in Tier alone.
	WantedTier string `json:"wantedTier,omitempty"`
	Candidates int    `json:"candidates,omitempty"`
	Model     string  `json:"model"`
	AccountID string  `json:"accountId"`
	Score     float64 `json:"score"`
	Explored  bool    `json:"explored"`
	Pinned    bool    `json:"pinned"`
	Thinking  bool    `json:"thinking"` // thinking switched on by the router
	// Display-only, filled in by Snapshot from the account list.
	Email    string       `json:"email,omitempty"`
	Provider string       `json:"provider,omitempty"`
	Signals  routeSignals `json:"signals"`
	Reason   string       `json:"reason"`
}

const decisionsRingSize = 200

type autoRouter struct {
	mu        sync.Mutex
	stats     map[string]*candidateStats // "accountID|model"
	decisions []routeDecision
	rng       *rand.Rand
	store     config.BlobStore
	dirty     bool
	// vision reports image support for a model ID (nil = unknown). When a
	// request carries images, candidates that are known to accept them win.
	vision func(model string) bool
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
	tier := effectiveTier(cfg, sig)
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
	// Pass 1 (vision requests only): a tier with image-capable candidates,
	// nearest tier first. Pass 2: any candidates. So a text-only tier never
	// beats a neighbouring tier that can actually see the image.
	var cands []routeCandidate
	usedTier := tier
	// anyFit holds the first non-empty tier ignoring the context check, so a
	// request larger than every known window still routes somewhere instead of
	// failing outright.
	var anyFit []routeCandidate
	anyFitTier := tier
	truncatedFit := false
	passes := []bool{false}
	if sig.Images && r.vision != nil {
		passes = []bool{true, false}
	}
search:
	for _, needVision := range passes {
		for _, ti := range order {
			cands = r.candidates(p, tiers[ti], filter, cfg)
			if needVision {
				cands = r.onlyVision(cands)
			}
			if len(cands) == 0 {
				continue
			}
			if len(anyFit) == 0 {
				anyFit, anyFitTier = cands, ti
			}
			if fits := onlyFitsContext(cands, sig.InputTokens); len(fits) > 0 {
				cands = fits
				usedTier = ti
				break search
			}
			truncatedFit = true
			cands = nil
		}
	}
	if len(cands) == 0 {
		if len(anyFit) == 0 {
			return nil
		}
		cands, usedTier = anyFit, anyFitTier
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
		score := reliability * (1 - cfg.SpeedWeight*(1-speed)) * (1 - cfg.CostWeight*(1-affordability(c)))
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
	if truncatedFit {
		// Every candidate with a known window was too small for this request;
		// the pick may not hold the whole context.
		reason += " context-overflow"
	}
	if usedTier != tier {
		reason += fmt.Sprintf(" (wanted %s)", tierNames[tier])
	}
	if explored {
		reason += " explore"
	}
	d := routeDecision{
		Time: now.Unix(), Endpoint: endpoint, Tier: tierNames[usedTier], Model: pick.model,
		AccountID: pick.account.ID, Score: math.Round(pickScore*1000) / 1000, Explored: explored,
		Candidates: len(cands), Signals: sig, Reason: reason,
	}
	if usedTier != tier {
		d.WantedTier = tierNames[tier]
	}
	r.pushLocked(d)
	return &d
}

// priceReference is the per-1M price treated as "mid-market": a model at this
// price scores 0.5 on affordability, free models score 1, and something ten
// times dearer lands near 0.09. Chosen so the curve discriminates across the
// range actually seen in the catalog ($0 to $30/1M) rather than saturating.
const priceReference = 2.0

// affordability scores what a candidate costs to use, from 1 (free and
// unthrottled) down towards 0.
//
// It weighs two unrelated meanings of "expensive" and takes the harsher:
// money (models.dev price per 1M tokens) and quota (how little of a metered
// account is left). Multiplying them would compound two independent penalties
// into a number that means neither; the minimum keeps the binding constraint
// legible. A model the catalog does not price is treated as mid-market — an
// unknown price is not evidence of a cheap one.
func affordability(c routeCandidate) float64 {
	money := 0.5
	if price, ok := modelsdev.BlendedPrice(c.model); ok {
		money = priceReference / (priceReference + price)
	}
	quota := 1.0
	if c.account != nil && c.account.UsageLimit > 0 {
		quota = math.Max(0, 1-c.account.UsagePercent)
	}
	return math.Min(money, quota)
}

// onlyVision keeps candidates whose model is known to accept images.
// contextReserve is the room left for the reply (and for the token estimate
// running low) when checking whether a request fits a model's window.
const contextReserve = 32_000

// onlyFitsContext drops candidates whose published context window cannot hold
// inputTokens plus the reply reserve. Models the models.dev catalog does not
// know are kept — an unknown window is not evidence of a small one, and
// dropping them would empty the pool for every provider-specific id.
func onlyFitsContext(cands []routeCandidate, inputTokens int) []routeCandidate {
	if inputTokens <= 0 {
		return cands
	}
	var out []routeCandidate
	for _, c := range cands {
		if modelFitsContext(c.model, inputTokens) {
			out = append(out, c)
		}
	}
	return out
}

// modelFitsContext reports whether model can hold inputTokens plus the reply
// reserve. Unknown windows count as fitting (see onlyFitsContext).
func modelFitsContext(model string, inputTokens int) bool {
	if inputTokens <= 0 {
		return true
	}
	limit := modelsdev.ContextLimit(model)
	return limit == 0 || limit >= inputTokens+contextReserve
}

func (r *autoRouter) onlyVision(cands []routeCandidate) []routeCandidate {
	var out []routeCandidate
	for _, c := range cands {
		if r.vision != nil && r.vision(c.model) {
			out = append(out, c)
		}
	}
	return out
}

// wantsThinking is the auto-thinking heuristic: heavy requests (the raw
// strong-tier signals: very long context or many tools) get reasoning on.
func wantsThinking(cfg config.AutoRouteConfig, sig routeSignals) bool {
	return cfg.AutoThinking && !sig.Thinking && classifyTier(sig) == 2
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
			if r.isQuarantined(acc.ID, m) {
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

// isQuarantined reports whether a pair is currently sidelined.
func (r *autoRouter) isQuarantined(accountID, model string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats[statKey(accountID, model)].quarantined(time.Now())
}

// RecordHardFailure sidelines a pair the upstream rejected in a way that will
// repeat: an unknown model, a malformed pairing, a revoked credential. Unlike
// a 429 (which the pool already routes around and which resolves on its own),
// retrying these burns a request to learn nothing.
func (r *autoRouter) RecordHardFailure(accountID, model, reason string) {
	if r == nil || accountID == "" || model == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := statKey(accountID, model)
	st := r.stats[k]
	if st == nil {
		st = &candidateStats{}
		r.stats[k] = st
	}
	st.QuarantinedUntil = time.Now().Add(quarantineWindow)
	st.QuarantineReason = reason
	r.dirty = true
	logger.Warnf("[AutoRoute] quarantined %s for %s (%s)", model, quarantineWindow, reason)
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
	// Quarantined pairs are skipped by the router; showing why (and until when)
	// keeps the exclusion auditable instead of looking like a silent snub.
	QuarantinedUntil int64  `json:"quarantinedUntil,omitempty"`
	QuarantineReason string `json:"quarantineReason,omitempty"`
}

// Snapshot returns recent decisions (newest first) and per-candidate stats.
func (r *autoRouter) Snapshot() ([]routeDecision, []candidateView) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	providers, emails := map[string]string{}, map[string]string{}
	for _, a := range config.GetAccounts() {
		emails[a.ID] = a.Email
		if prov, err := config.ProviderForAccount(&a); err == nil {
			providers[a.ID] = string(prov)
		}
	}
	dec := make([]routeDecision, len(r.decisions))
	for i, d := range r.decisions {
		d.Email, d.Provider = emails[d.AccountID], providers[d.AccountID]
		dec[len(r.decisions)-1-i] = d
	}
	var cands []candidateView
	for k, st := range r.stats {
		st.decay(now)
		parts := strings.SplitN(k, "|", 2)
		view := candidateView{
			AccountID: parts[0], Provider: providers[parts[0]], Email: emails[parts[0]], Model: parts[1],
			Successes: math.Round(st.Successes*100) / 100, Failures: math.Round(st.Failures*100) / 100,
			EwmaLatency: math.Round(st.EwmaLatency),
			Reliability: math.Round((st.Successes+1)/(st.Successes+st.Failures+2)*1000) / 1000,
			LastUpdated: st.Updated.Unix(),
		}
		if st.quarantined(now) {
			view.QuarantinedUntil = st.QuarantinedUntil.Unix()
			view.QuarantineReason = truncateReason(st.QuarantineReason)
		}
		cands = append(cands, view)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Reliability > cands[j].Reliability })
	return dec, cands
}

// resolveAutoModel applies the router to a request for the virtual "auto"
// model. It returns the concrete model to use and pins the chosen account on
// the affinity key (creating a per-request key if the conversation has none)
// so the normal pickAccount path selects it. Headers announce the decision.
// The second result reports that the router switched thinking on for a plain
// "auto" request (see wantsThinking); callers must honour it.
func (h *Handler) resolveAutoModel(w http.ResponseWriter, endpoint, model string, sig routeSignals, affinityKey *string, cap providerCapability) (string, bool) {
	if !isAutoModel(model) {
		return model, false
	}
	cfg := config.GetAutoRouteConfig()
	if !cfg.Enabled {
		return model, false
	}
	think := wantsThinking(cfg, sig)
	reasonSuffix := ""
	if think {
		reasonSuffix = "; thinking=auto"
	}
	// Cache affinity wins: a pinned conversation keeps its account and model.
	if *affinityKey != "" {
		if accID, pinnedModel := h.affinity.GetWithModel(*affinityKey); accID != "" && pinnedModel != "" && !isAutoModel(pinnedModel) {
			// A pin survives only while it still suits the conversation. It is
			// dropped when the context outgrew the model's window, when the
			// request now classifies above the pinned model's tier (a session
			// that opened with "hi" and turned into a refactor), or when the
			// pair has been quarantined. Otherwise the pin — which decides the
			// overwhelming majority of requests — would outlive its own premise.
			keep := h.pool.GetForModelByID(accID, pinnedModel, capabilityFilter(cap)) != nil &&
				modelFitsContext(pinnedModel, sig.InputTokens) &&
				!h.autoRouter.isQuarantined(accID, pinnedModel) &&
				!pinOutranked(cfg, pinnedModel, sig)
			if keep {
				h.autoRouter.RecordPinned(routeDecision{Time: time.Now().Unix(), Endpoint: endpoint, Tier: "pinned", Model: pinnedModel, AccountID: accID, Pinned: true, Thinking: think, Signals: sig, Reason: "conversation pinned (warm cache)" + reasonSuffix})
				w.Header().Set("X-Hekato-Routed-Model", pinnedModel)
				w.Header().Set("X-Hekato-Route-Reason", "pinned"+reasonSuffix)
				return pinnedModel, think
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
			w.Header().Set("X-Hekato-Route-Reason", "no tier candidates; fallback to advertised model"+reasonSuffix)
			h.autoRouter.RecordPinned(routeDecision{Time: time.Now().Unix(), Endpoint: endpoint, Tier: "fallback", Model: fb, Thinking: think, Signals: sig, Reason: "no tier candidates; first advertised model" + reasonSuffix})
			return fb, think
		}
		logger.Warnf("[AutoRoute] no routable model advertised for an auto request; passing \"auto\" upstream")
		return model, false
	}
	if think {
		d.Thinking = true
		d.Reason += reasonSuffix
		h.autoRouter.markThinking(d.Time, d.AccountID, d.Model)
	}
	if *affinityKey == "" {
		// Per-request key so pickAccount honours the router's choice; uuid is
		// goroutine-safe (the router's rand.Rand is only used under its mutex).
		*affinityKey = "auto:" + uuid.NewString()
	}
	h.affinity.SetWithModel(*affinityKey, d.AccountID, d.Model)
	w.Header().Set("X-Hekato-Routed-Model", d.Model)
	w.Header().Set("X-Hekato-Route-Reason", d.Reason)
	return d.Model, think
}

// markThinking flags the most recent matching decision in the ring (Resolve
// pushed a copy before the caller knew whether thinking would be enabled).
func (r *autoRouter) markThinking(ts int64, accountID, model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.decisions) - 1; i >= 0; i-- {
		d := &r.decisions[i]
		if d.Time == ts && d.AccountID == accountID && d.Model == model {
			d.Thinking = true
			d.Reason += "; thinking=auto"
			return
		}
	}
}

// pinOutranked reports whether the request now classifies above the tier of
// the pinned model. A model no tier claims (-1) is left alone: an operator who
// pinned something outside the tier lists did so deliberately, and guessing
// would churn pins for no gain.
func pinOutranked(cfg config.AutoRouteConfig, pinnedModel string, sig routeSignals) bool {
	pinnedTier := tierOfModel(cfg, pinnedModel)
	if pinnedTier < 0 {
		return false
	}
	return effectiveTier(cfg, sig) > pinnedTier
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
	s.applyContent(lastClaudeUserText(req))
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
	s.applyContent(lastOpenAIUserText(req))
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

// applyContent folds the content signals of the last user message into s.
// The text is analysed and discarded; only the resulting flags are kept.
func (s *routeSignals) applyContent(text string) {
	c := analyseText(text, config.GetAutoRouteConfig())
	s.Code, s.Reasoning, s.Simple = c.Code, c.Reasoning, c.Simple
	if c.KeywordTier >= 0 {
		s.KeywordTier = c.KeywordTier + 1 // 0 stays "no rule matched"
	}
}

// lastClaudeUserText flattens the final user message to plain text. Tool
// results are skipped: a tool-output turn says nothing about how hard the
// user's actual request is.
func lastClaudeUserText(req *ClaudeRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		m := req.Messages[i]
		if !strings.EqualFold(m.Role, "user") {
			continue
		}
		switch v := m.Content.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return truncateForAnalysis(v)
			}
		case []interface{}:
			var b strings.Builder
			for _, blk := range v {
				bm, ok := blk.(map[string]interface{})
				if !ok || bm["type"] != "text" {
					continue // tool_result / image blocks carry no user intent
				}
				if t, ok := bm["text"].(string); ok {
					b.WriteString(t)
					b.WriteByte('\n')
				}
			}
			if strings.TrimSpace(b.String()) != "" {
				return truncateForAnalysis(b.String())
			}
		}
	}
	return ""
}

// lastOpenAIUserText is lastClaudeUserText for the OpenAI message shape.
func lastOpenAIUserText(req *OpenAIRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		m := req.Messages[i]
		if !strings.EqualFold(m.Role, "user") {
			continue
		}
		switch v := m.Content.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return truncateForAnalysis(v)
			}
		case []interface{}:
			var b strings.Builder
			for _, part := range v {
				pm, ok := part.(map[string]interface{})
				if !ok {
					continue
				}
				if t, ok := pm["text"].(string); ok {
					b.WriteString(t)
					b.WriteByte('\n')
				}
			}
			if strings.TrimSpace(b.String()) != "" {
				return truncateForAnalysis(b.String())
			}
		}
	}
	return ""
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decisions":  dec,
		"candidates": cands,
		"tierHealth": h.tierHealth(dec),
	})
}

// truncateReason keeps an upstream rejection readable in a tooltip.
func truncateReason(s string) string {
	const max = 160
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// tierHealthRow is one tier's supply and demand: how many candidates it can
// offer right now, how often requests asked for it, and how often it could not
// serve them and a neighbouring tier stepped in.
type tierHealthRow struct {
	Tier       string `json:"tier"`
	Candidates int    `json:"candidates"`
	Wanted     int    `json:"wanted"`
	Served     int    `json:"served"`
	Starved    int    `json:"starved"`
}

// tierHealth answers the question the decision ring could only hint at: a tier
// whose requests keep landing elsewhere is misconfigured, not unlucky.
func (h *Handler) tierHealth(dec []routeDecision) []tierHealthRow {
	cfg := config.GetAutoRouteConfig()
	rows := make([]tierHealthRow, 3)
	for i, patterns := range [3][]string{cfg.Fast, cfg.Balanced, cfg.Strong} {
		rows[i] = tierHealthRow{
			Tier:       tierNames[i],
			Candidates: len(h.autoRouter.candidates(h.pool, patterns, nil, cfg)),
		}
	}
	index := map[string]int{tierNames[0]: 0, tierNames[1]: 1, tierNames[2]: 2}
	for _, d := range dec {
		if i, ok := index[d.Tier]; ok {
			rows[i].Served++
			// A decision with no WantedTier wanted the tier that served it.
			if d.WantedTier == "" {
				rows[i].Wanted++
			}
		}
		if i, ok := index[d.WantedTier]; ok {
			rows[i].Wanted++
			rows[i].Starved++
		}
	}
	return rows
}
