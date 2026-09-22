package proxy

import (
	"encoding/json"
	"hekato-go/config"
	"hekato-go/logger"
	"net/http"
	"sort"
	"sync"
	"time"
)

// metricsCollector keeps 7 days of per-minute buckets for the ops dashboard:
// request/error counts, tokens, credits, latency histogram, and per-model /
// per-account / per-endpoint breakdowns. Buckets are persisted through
// config.Metrics() (SQL table or metrics.json) so history survives restarts:
// loaded once at startup, dirty minutes flushed every 30s and on shutdown.

const metricsMinutes = 7 * 24 * 60

// latency histogram edges in ms; the last bucket is open-ended.
var latencyEdges = []int64{250, 500, 1000, 2000, 4000, 8000, 16000, 32000, 64000}

type dimCounter struct {
	Requests int     `json:"requests"`
	Errors   int     `json:"errors"`
	Tokens   int     `json:"tokens"`
	Credits  float64 `json:"credits"`
	Latency  int64   `json:"latency,omitempty"`
}

type metricsBucket struct {
	minute     int64 // unix minute
	requests   int
	errors     int
	tokens     int
	credits    float64
	latencySum int64
	latencyN   int
	hist       [10]int
	byModel    map[string]*dimCounter
	byAccount  map[string]*dimCounter
	byEndpoint map[string]*dimCounter
}

// bucketRecord is the persisted form of a bucket.
type bucketRecord struct {
	Requests   int                    `json:"r"`
	Errors     int                    `json:"e"`
	Tokens     int                    `json:"t"`
	Credits    float64                `json:"c"`
	LatencySum int64                  `json:"ls"`
	LatencyN   int                    `json:"ln"`
	Hist       [10]int                `json:"h"`
	ByModel    map[string]*dimCounter `json:"m,omitempty"`
	ByAccount  map[string]*dimCounter `json:"a,omitempty"`
	ByEndpoint map[string]*dimCounter `json:"ep,omitempty"`
}

func (b *metricsBucket) record() bucketRecord {
	return bucketRecord{Requests: b.requests, Errors: b.errors, Tokens: b.tokens, Credits: b.credits,
		LatencySum: b.latencySum, LatencyN: b.latencyN, Hist: b.hist, ByModel: b.byModel, ByAccount: b.byAccount, ByEndpoint: b.byEndpoint}
}

func bucketFromRecord(minute int64, r bucketRecord) *metricsBucket {
	b := &metricsBucket{minute: minute, requests: r.Requests, errors: r.Errors, tokens: r.Tokens, credits: r.Credits,
		latencySum: r.LatencySum, latencyN: r.LatencyN, hist: r.Hist, byModel: r.ByModel, byAccount: r.ByAccount, byEndpoint: r.ByEndpoint}
	if b.byModel == nil {
		b.byModel = map[string]*dimCounter{}
	}
	if b.byAccount == nil {
		b.byAccount = map[string]*dimCounter{}
	}
	if b.byEndpoint == nil {
		b.byEndpoint = map[string]*dimCounter{}
	}
	return b
}

type metricsCollector struct {
	mu      sync.Mutex
	buckets [metricsMinutes]*metricsBucket
	dirty   map[int64]bool
	store   config.MetricsStore
}

func newMetricsCollector() *metricsCollector {
	return &metricsCollector{dirty: map[int64]bool{}}
}

// Load attaches the persistence backend and restores the retained window.
func (m *metricsCollector) Load(store config.MetricsStore) {
	if m == nil || store == nil {
		return
	}
	since := time.Now().Unix()/60 - metricsMinutes + 1
	rows, err := store.LoadMetrics(since)
	if err != nil {
		logger.Warnf("[Metrics] load failed: %v", err)
		return
	}
	m.mu.Lock()
	m.store = store
	restored := 0
	for minute, data := range rows {
		if minute < since {
			continue
		}
		var rec bucketRecord
		if json.Unmarshal([]byte(data), &rec) != nil {
			continue
		}
		m.buckets[int(minute%metricsMinutes)] = bucketFromRecord(minute, rec)
		restored++
	}
	m.mu.Unlock()
	logger.Infof("[Metrics] restored %d minute buckets", restored)
}

// Flush persists dirty buckets and prunes rows outside the retention window.
func (m *metricsCollector) Flush() {
	if m == nil {
		return
	}
	m.mu.Lock()
	store := m.store
	if store == nil || len(m.dirty) == 0 {
		m.mu.Unlock()
		return
	}
	rows := make(map[int64]string, len(m.dirty))
	for minute := range m.dirty {
		b := m.buckets[int(minute%metricsMinutes)]
		if b == nil || b.minute != minute {
			continue
		}
		data, err := json.Marshal(b.record())
		if err == nil {
			rows[minute] = string(data)
		}
	}
	m.dirty = map[int64]bool{}
	m.mu.Unlock()

	if err := store.SaveMetrics(rows); err != nil {
		logger.Warnf("[Metrics] save failed: %v", err)
		m.mu.Lock()
		for minute := range rows {
			m.dirty[minute] = true // retry next flush
		}
		m.mu.Unlock()
		return
	}
	if err := store.PruneMetrics(time.Now().Unix()/60 - metricsMinutes); err != nil {
		logger.Warnf("[Metrics] prune failed: %v", err)
	}
}

// run flushes every 30s until stop is closed, then flushes once more.
func (m *metricsCollector) run(stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.Flush()
		case <-stop:
			m.Flush()
			return
		}
	}
}

func (m *metricsCollector) bucketFor(minute int64) *metricsBucket {
	idx := int(minute % metricsMinutes)
	b := m.buckets[idx]
	if b == nil || b.minute != minute {
		b = &metricsBucket{minute: minute, byModel: map[string]*dimCounter{}, byAccount: map[string]*dimCounter{}, byEndpoint: map[string]*dimCounter{}}
		m.buckets[idx] = b
	}
	return b
}

func bump(mp map[string]*dimCounter, key string, ok bool, tokens int, credits float64, latency int64) {
	if key == "" {
		key = "-"
	}
	c := mp[key]
	if c == nil {
		c = &dimCounter{}
		mp[key] = c
	}
	c.Requests++
	if !ok {
		c.Errors++
	}
	c.Tokens += tokens
	c.Credits += credits
	c.Latency += latency
}

func (m *metricsCollector) Record(endpoint, model, accountID string, ok bool, tokens int, credits float64, latencyMs int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	minute := time.Now().Unix() / 60
	b := m.bucketFor(minute)
	m.dirty[minute] = true
	b.requests++
	if !ok {
		b.errors++
	}
	b.tokens += tokens
	b.credits += credits
	if ok && latencyMs > 0 {
		b.latencySum += latencyMs
		b.latencyN++
		i := 0
		for i < len(latencyEdges) && latencyMs > latencyEdges[i] {
			i++
		}
		b.hist[i]++
	}
	bump(b.byModel, model, ok, tokens, credits, latencyMs)
	bump(b.byAccount, accountID, ok, tokens, credits, latencyMs)
	bump(b.byEndpoint, endpoint, ok, tokens, credits, latencyMs)
}

type metricsPoint struct {
	Time       int64   `json:"t"`
	Requests   int     `json:"requests"`
	Errors     int     `json:"errors"`
	Tokens     int     `json:"tokens"`
	Credits    float64 `json:"credits"`
	AvgLatency int64   `json:"avgLatencyMs"`
	P50        int64   `json:"p50Ms"`
	P95        int64   `json:"p95Ms"`
}

type dimRow struct {
	Key        string  `json:"key"`
	Requests   int     `json:"requests"`
	Errors     int     `json:"errors"`
	Tokens     int     `json:"tokens"`
	Credits    float64 `json:"credits"`
	AvgLatency int64   `json:"avgLatencyMs"`
}

func percentileFromHist(hist [10]int, n int, q float64) int64 {
	if n == 0 {
		return 0
	}
	target := int(float64(n)*q + 0.5)
	if target < 1 {
		target = 1
	}
	acc := 0
	for i, c := range hist {
		acc += c
		if acc >= target {
			if i < len(latencyEdges) {
				return latencyEdges[i]
			}
			return latencyEdges[len(latencyEdges)-1] * 2
		}
	}
	return 0
}

func rowsFrom(mp map[string]*dimCounter, limit int) []dimRow {
	rows := make([]dimRow, 0, len(mp))
	for k, c := range mp {
		r := dimRow{Key: k, Requests: c.Requests, Errors: c.Errors, Tokens: c.Tokens, Credits: c.Credits}
		if okN := c.Requests - c.Errors; okN > 0 {
			r.AvgLatency = c.Latency / int64(okN)
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Requests > rows[j].Requests })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// Query aggregates the last `minutes` minutes into points of `step` minutes.
func (m *metricsCollector) Query(minutes, step int) (points []metricsPoint, totals metricsPoint, byModel, byAccount, byEndpoint []dimRow) {
	if minutes > metricsMinutes {
		minutes = metricsMinutes
	}
	if step < 1 {
		step = 1
	}
	now := time.Now().Unix() / 60
	start := now - int64(minutes) + 1
	agModel, agAccount, agEndpoint := map[string]*dimCounter{}, map[string]*dimCounter{}, map[string]*dimCounter{}
	var totalHist [10]int

	m.mu.Lock()
	defer m.mu.Unlock()
	for from := start; from <= now; from += int64(step) {
		p := metricsPoint{Time: from * 60}
		var hist [10]int
		var latN int
		for minute := from; minute < from+int64(step) && minute <= now; minute++ {
			b := m.buckets[int(minute%metricsMinutes)]
			if b == nil || b.minute != minute {
				continue
			}
			p.Requests += b.requests
			p.Errors += b.errors
			p.Tokens += b.tokens
			p.Credits += b.credits
			p.AvgLatency += b.latencySum
			latN += b.latencyN
			for i := range hist {
				hist[i] += b.hist[i]
				totalHist[i] += b.hist[i]
			}
			for k, c := range b.byModel {
				bumpAgg(agModel, k, c)
			}
			for k, c := range b.byAccount {
				bumpAgg(agAccount, k, c)
			}
			for k, c := range b.byEndpoint {
				bumpAgg(agEndpoint, k, c)
			}
		}
		totals.Requests += p.Requests
		totals.Errors += p.Errors
		totals.Tokens += p.Tokens
		totals.Credits += p.Credits
		totals.AvgLatency += p.AvgLatency
		totals.P50 += int64(latN) // temporarily count
		if latN > 0 {
			p.AvgLatency /= int64(latN)
		}
		p.P50 = percentileFromHist(hist, latN, 0.5)
		p.P95 = percentileFromHist(hist, latN, 0.95)
		points = append(points, p)
	}
	totalN := int(totals.P50)
	if totalN > 0 {
		totals.AvgLatency /= int64(totalN)
	}
	totals.P50 = percentileFromHist(totalHist, totalN, 0.5)
	totals.P95 = percentileFromHist(totalHist, totalN, 0.95)
	return points, totals, rowsFrom(agModel, 20), rowsFrom(agAccount, 20), rowsFrom(agEndpoint, 10)
}

func bumpAgg(mp map[string]*dimCounter, key string, c *dimCounter) {
	a := mp[key]
	if a == nil {
		a = &dimCounter{}
		mp[key] = a
	}
	a.Requests += c.Requests
	a.Errors += c.Errors
	a.Tokens += c.Tokens
	a.Credits += c.Credits
	a.Latency += c.Latency
}

// apiGetMetrics: GET /admin/api/metrics?range=1h|6h|24h|7d
func (h *Handler) apiGetMetrics(w http.ResponseWriter, r *http.Request) {
	minutes, step := 60, 1
	switch r.URL.Query().Get("range") {
	case "6h":
		minutes, step = 360, 5
	case "24h":
		minutes, step = 1440, 15
	case "7d":
		minutes, step = 7*1440, 60
	}
	points, totals, byModel, byAccount, byEndpoint := h.metrics.Query(minutes, step)
	if points == nil {
		points = []metricsPoint{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rangeMinutes": minutes,
		"stepMinutes":  step,
		"series":       points,
		"totals":       totals,
		"byModel":      byModel,
		"byAccount":    byAccount,
		"byEndpoint":   byEndpoint,
	})
}
