package proxy

import (
	"encoding/json"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers/modelsdev"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// modelHealth rolls request outcomes up per (account, model) for the model
// status page: hourly buckets for the last day (success, TTFT, speed) and
// daily buckets for the 60-day strip. The request log keeps only 500 entries,
// far too few for either window.

const (
	modelHealthBlobKey = "modelhealth"
	healthHours        = 24
	healthDays         = 60
)

// ttftEdges are the TTFT histogram bucket upper bounds in ms (last bucket open).
var ttftEdges = []int64{500, 1000, 1500, 2000, 3000, 4000, 5000, 6500, 8000, 10000, 12500, 15000, 20000, 25000, 30000, 45000, 60000}

type healthCounter struct {
	Requests int     `json:"r"`
	Errors   int     `json:"e,omitempty"`
	TTFT     []int   `json:"tt,omitempty"` // histogram over ttftEdges (hourly buckets only)
	TPSSum   float64 `json:"ts,omitempty"`
	TPSN     int     `json:"tn,omitempty"`
}

type modelHealthState struct {
	Hours map[int64]map[string]*healthCounter `json:"hours"` // unix hour → statKey → counter
	Days  map[int64]map[string]*healthCounter `json:"days"`  // unix day → statKey → counter
}

type modelHealth struct {
	mu    sync.Mutex
	state modelHealthState
	dirty bool
	store config.BlobStore
}

func newModelHealth() *modelHealth {
	return &modelHealth{state: modelHealthState{Hours: map[int64]map[string]*healthCounter{}, Days: map[int64]map[string]*healthCounter{}}}
}

func (m *modelHealth) Load(store config.BlobStore) {
	if m == nil || store == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = store
	data, err := store.LoadBlob(modelHealthBlobKey)
	if err != nil || data == "" {
		return
	}
	var st modelHealthState
	if err := json.Unmarshal([]byte(data), &st); err != nil {
		logger.Warnf("[ModelHealth] load: bad state: %v", err)
		return
	}
	if st.Hours != nil {
		m.state.Hours = st.Hours
	}
	if st.Days != nil {
		m.state.Days = st.Days
	}
}

func (m *modelHealth) Flush() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.store == nil || !m.dirty {
		m.mu.Unlock()
		return
	}
	m.prune(time.Now())
	data, err := json.Marshal(m.state)
	m.dirty = false
	store := m.store
	m.mu.Unlock()
	if err == nil {
		if err := store.SaveBlob(modelHealthBlobKey, string(data)); err != nil {
			logger.Warnf("[ModelHealth] save failed: %v", err)
		}
	}
}

func (m *modelHealth) prune(now time.Time) {
	for h := range m.state.Hours {
		if h <= now.Unix()/3600-healthHours {
			delete(m.state.Hours, h)
		}
	}
	for d := range m.state.Days {
		if d <= now.Unix()/86400-healthDays {
			delete(m.state.Days, d)
		}
	}
}

func counterIn(buckets map[int64]map[string]*healthCounter, at int64, key string) *healthCounter {
	b := buckets[at]
	if b == nil {
		b = map[string]*healthCounter{}
		buckets[at] = b
	}
	c := b[key]
	if c == nil {
		c = &healthCounter{}
		b[key] = c
	}
	return c
}

// Record folds one request-log entry in.
func (m *modelHealth) Record(e RequestLog) {
	if m == nil || e.AccountID == "" || e.Model == "" {
		return
	}
	key := statKey(e.AccountID, e.Model)
	ok := e.Status == "success"
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirty = true
	day := counterIn(m.state.Days, now.Unix()/86400, key)
	hour := counterIn(m.state.Hours, now.Unix()/3600, key)
	for _, c := range []*healthCounter{day, hour} {
		c.Requests++
		if !ok {
			c.Errors++
		}
	}
	if ok && e.TTFTMs > 0 {
		if hour.TTFT == nil {
			hour.TTFT = make([]int, len(ttftEdges)+1)
		}
		i := sort.Search(len(ttftEdges), func(i int) bool { return e.TTFTMs <= ttftEdges[i] })
		hour.TTFT[i]++
	}
	if ok && e.TPS > 0 {
		hour.TPSSum += e.TPS
		hour.TPSN++
	}
}

// ttftP50 interpolates the median inside its histogram bucket.
func ttftP50(hist []int) int64 {
	n := 0
	for _, c := range hist {
		n += c
	}
	if n == 0 {
		return 0
	}
	half := float64(n) / 2
	acc := 0.0
	for i, c := range hist {
		if c == 0 {
			continue
		}
		if acc+float64(c) >= half {
			lo := int64(0)
			if i > 0 {
				lo = ttftEdges[i-1]
			}
			hi := lo * 2 // open-ended last bucket
			if i < len(ttftEdges) {
				hi = ttftEdges[i]
			}
			return lo + int64((half-acc)/float64(c)*float64(hi-lo))
		}
		acc += float64(c)
	}
	return 0
}

// healthStatus labels a success rate the way the status page legend does.
func healthStatus(requests, errors int) string {
	if requests == 0 {
		return "none"
	}
	switch rate := float64(requests-errors) / float64(requests); {
	case rate >= 0.9:
		return "operational"
	case rate >= 0.6:
		return "degraded"
	default:
		return "outage"
	}
}

type healthRoute struct {
	Route       string   `json:"route"` // "<pool prefix>/<model>"
	Pool        string   `json:"pool"`
	Model       string   `json:"model"`
	Tiers       []string `json:"tiers"`
	Accounts    int      `json:"accounts"`
	Status      string   `json:"status"`
	Requests    int      `json:"requests"`
	Errors      int      `json:"errors"`
	TTFTP50     int64    `json:"ttftP50"`
	TPS         float64  `json:"tps"`
	Context     int      `json:"context"`
	MaxOutput   int      `json:"maxOutput"`
	Slow        bool     `json:"slow,omitempty"`
	Quarantined bool     `json:"quarantined,omitempty"`
	Days        []string `json:"days"` // status per day, oldest first (healthDays)
}

type healthDay struct {
	Day      int64  `json:"day"` // unix seconds, UTC midnight
	Status   string `json:"status"`
	Requests int    `json:"requests"`
	Errors   int    `json:"errors"`
}

// apiModelHealth reports every (pool, model) the auto-route tiers cover, with
// its last-24h health and a 60-day daily strip for the whole pool.
func (h *Handler) apiModelHealth(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetAutoRouteConfig()
	tiers := [3][]string{cfg.Fast, cfg.Balanced, cfg.Strong}
	known := advertisedModels(h.pool)
	prefixes := config.GetPoolPrefixes()
	now := time.Now()

	routes := map[string]*healthRoute{}
	keys := map[string][]string{} // route → statKeys of its (account, model) pairs
	for _, acc := range h.pool.GetAllAccounts() {
		poolKey := config.PoolKey(&acc)
		prefix := prefixes[poolKey]
		if prefix == "" {
			prefix = poolKey
		}
		prov, _ := config.ProviderForAccount(&acc)
		var enabled map[string]bool
		if list := h.pool.EnabledModels(acc.ID); list != nil {
			enabled = map[string]bool{}
			for _, m := range list {
				enabled[m] = true
			}
		}
		for _, model := range h.pool.GetModelList(acc.ID) {
			if isAutoModel(model) || enabled != nil && !enabled[model] || cfg.Blacklisted(prov, poolKey, model) {
				continue
			}
			var in []string
			for ti, pats := range tiers {
				if matchesTierIn(poolKey, model, pats, known) {
					in = append(in, tierNames[ti])
				}
			}
			if len(in) == 0 {
				continue
			}
			name := prefix + "/" + model
			rt := routes[name]
			if rt == nil {
				rt = &healthRoute{Route: name, Pool: prefix, Model: model, Tiers: in,
					Context: modelsdev.ContextLimit(model), MaxOutput: modelsdev.OutputLimit(model)}
				routes[name] = rt
			}
			rt.Accounts++
			rt.Slow = rt.Slow || h.autoRouter.isSlow(acc.ID, model)
			rt.Quarantined = rt.Quarantined || h.autoRouter.isQuarantined(acc.ID, model)
			keys[name] = append(keys[name], statKey(acc.ID, model))
		}
	}

	today := now.Unix() / 86400
	poolDays := make([]healthDay, healthDays)
	for i := range poolDays {
		poolDays[i].Day = (today - healthDays + 1 + int64(i)) * 86400
	}
	h.modelHealth.mu.Lock()
	for name, rt := range routes {
		var hist []int
		tpsN := 0
		for hr := now.Unix()/3600 - healthHours + 1; hr <= now.Unix()/3600; hr++ {
			for _, k := range keys[name] {
				c := h.modelHealth.state.Hours[hr][k]
				if c == nil {
					continue
				}
				rt.Requests += c.Requests
				rt.Errors += c.Errors
				rt.TPS += c.TPSSum
				tpsN += c.TPSN
				if hist == nil {
					hist = make([]int, len(ttftEdges)+1)
				}
				for i, n := range c.TTFT {
					hist[i] += n
				}
			}
		}
		if tpsN > 0 {
			rt.TPS = math.Round(rt.TPS/float64(tpsN)*10) / 10
		}
		rt.TTFTP50 = ttftP50(hist)
		rt.Status = healthStatus(rt.Requests, rt.Errors)
		rt.Days = make([]string, healthDays)
		for i := range rt.Days {
			d := today - healthDays + 1 + int64(i)
			req, errs := 0, 0
			for _, k := range keys[name] {
				if c := h.modelHealth.state.Days[d][k]; c != nil {
					req += c.Requests
					errs += c.Errors
				}
			}
			rt.Days[i] = healthStatus(req, errs)
			poolDays[i].Requests += req
			poolDays[i].Errors += errs
		}
	}
	h.modelHealth.mu.Unlock()

	// The pool's status is its overall success rate, held at "degraded" while
	// any single route is struggling: the router steers around a failing
	// route, so one bad model does not make the whole pool an outage.
	out := make([]*healthRoute, 0, len(routes))
	total, failed, struggling := 0, 0, false
	for _, rt := range routes {
		out = append(out, rt)
		total += rt.Requests
		failed += rt.Errors
		struggling = struggling || rt.Status == "degraded" || rt.Status == "outage"
	}
	overall := healthStatus(total, failed)
	if overall == "operational" && struggling {
		overall = "degraded"
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Requests != out[j].Requests {
			return out[i].Requests > out[j].Requests
		}
		return strings.ToLower(out[i].Route) < strings.ToLower(out[j].Route)
	})
	for i := range poolDays {
		poolDays[i].Status = healthStatus(poolDays[i].Requests, poolDays[i].Errors)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"updated": now.Unix(),
		"status":  overall,
		"routes":  out,
		"days":    poolDays,
	})
}
