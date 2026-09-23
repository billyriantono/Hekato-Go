// Package modelsdev mirrors the public models.dev catalog: per-model pricing,
// context/output limits and capability flags for every provider it tracks.
// The admin panel renders it, and cost-aware routing can price a model with
// Find() without any account credentials — the feed is public and unauthenticated.
package modelsdev

import (
	"encoding/json"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	catalogURL = "https://models.dev/api.json"
	// blobKey names the persisted catalog in the runtime blob store, so a
	// restart serves prices immediately instead of re-pulling ~5 MB.
	blobKey = "models-dev-catalog"
	// defaultSyncHours is used when the admin has not set an interval.
	defaultSyncHours = 12
)

// Model is one flattened provider/model row. Prices are USD per 1M tokens.
type Model struct {
	Provider     string   `json:"provider"`     // provider id, e.g. "opencode"
	ProviderName string   `json:"providerName"` // display name, e.g. "OpenCode Zen"
	ID           string   `json:"id"`           // model id as sent to the API
	Name         string   `json:"name"`
	Input        float64  `json:"input"`
	Output       float64  `json:"output"`
	CacheRead    float64  `json:"cacheRead"`
	CacheWrite   float64  `json:"cacheWrite"`
	Free         bool     `json:"free"` // zero input AND output price
	ContextLimit int      `json:"contextLimit"`
	OutputLimit  int      `json:"outputLimit"`
	Reasoning    bool     `json:"reasoning"`
	ToolCall     bool     `json:"toolCall"`
	Attachment   bool     `json:"attachment"`
	Modalities   []string `json:"modalities,omitempty"` // input modalities
	ReleaseDate  string   `json:"releaseDate,omitempty"`
	Knowledge    string   `json:"knowledge,omitempty"`
}

// rawCatalog is the shape of models.dev/api.json: provider id → provider.
type rawCatalog map[string]struct {
	Name   string `json:"name"`
	Doc    string `json:"doc"`
	Models map[string]struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Cost struct {
			Input      float64 `json:"input"`
			Output     float64 `json:"output"`
			CacheRead  float64 `json:"cache_read"`
			CacheWrite float64 `json:"cache_write"`
		} `json:"cost"`
		Limit struct {
			Context int `json:"context"`
			Output  int `json:"output"`
		} `json:"limit"`
		Modalities struct {
			Input []string `json:"input"`
		} `json:"modalities"`
		Reasoning   bool   `json:"reasoning"`
		ToolCall    bool   `json:"tool_call"`
		Attachment  bool   `json:"attachment"`
		ReleaseDate string `json:"release_date"`
		Knowledge   string `json:"knowledge"`
	} `json:"models"`
}

var (
	mu         sync.Mutex
	cache      []Model
	fetchedAt  time.Time
	blobLoaded bool
)

// blob is the persisted shape of the catalog.
type blob struct {
	FetchedAt int64   `json:"fetchedAt"`
	Models    []Model `json:"models"`
}

// SyncInterval is how often the background sync re-pulls models.dev.
// Admin setting wins; 0 disables the periodic sync (on-demand only).
func SyncInterval() time.Duration {
	h := config.GetModelsDevSyncHours()
	if h < 0 {
		return 0
	}
	if h == 0 {
		h = defaultSyncHours
	}
	return time.Duration(h) * time.Hour
}

// List returns the cached catalog, pulling only when the cache (memory, then
// the persisted blob) is empty or older than the sync interval. On a failed
// refresh the stale cache is served rather than an error — pricing that is
// hours old still beats no pricing at all.
func List() ([]Model, time.Time, error) {
	mu.Lock()
	defer mu.Unlock()
	loadBlobLocked()
	ttl := SyncInterval()
	if len(cache) > 0 && (ttl == 0 || time.Since(fetchedAt) < ttl) {
		return cache, fetchedAt, nil
	}
	models, err := fetch()
	if err != nil {
		if len(cache) > 0 {
			return cache, fetchedAt, nil
		}
		return nil, time.Time{}, err
	}
	cache, fetchedAt = models, time.Now()
	saveBlobLocked()
	return cache, fetchedAt, nil
}

// Refresh forces a pull, ignoring the cache age.
func Refresh() ([]Model, time.Time, error) {
	mu.Lock()
	fetchedAt = time.Time{}
	blobLoaded = true // do not let the stale blob re-seed the timestamp
	mu.Unlock()
	return List()
}

// StartSync keeps the catalog warm in the background until stop closes.
// It re-reads the interval every cycle so an admin change takes effect without
// a restart.
func StartSync(stop <-chan struct{}) {
	for {
		if _, _, err := List(); err != nil {
			logger.Warnf("[models.dev] sync failed: %v", err)
		}
		wait := SyncInterval()
		if wait <= 0 {
			wait = time.Hour // sync disabled: idle, but keep watching for a setting change
		}
		select {
		case <-time.After(wait):
		case <-stop:
			return
		}
	}
}

// loadBlobLocked seeds the in-memory cache from storage once per process.
func loadBlobLocked() {
	if blobLoaded {
		return
	}
	blobLoaded = true
	bs := config.Blobs()
	if bs == nil {
		return
	}
	data, err := bs.LoadBlob(blobKey)
	if err != nil || data == "" {
		return
	}
	var b blob
	if err := json.Unmarshal([]byte(data), &b); err != nil || len(b.Models) == 0 {
		return
	}
	cache, fetchedAt = b.Models, time.Unix(b.FetchedAt, 0)
}

func saveBlobLocked() {
	bs := config.Blobs()
	if bs == nil {
		return
	}
	data, err := json.Marshal(blob{FetchedAt: fetchedAt.Unix(), Models: cache})
	if err != nil {
		return
	}
	if err := bs.SaveBlob(blobKey, string(data)); err != nil {
		logger.Warnf("[models.dev] persist catalog failed: %v", err)
	}
}

// Find looks up one model's metadata, e.g. to price a routing candidate.
// Pass an empty provider to match the model id across every provider.
func Find(provider, modelID string) (Model, bool) {
	models, _, err := List()
	if err != nil {
		return Model{}, false
	}
	for _, m := range models {
		if m.ID == modelID && (provider == "" || m.Provider == provider) {
			return m, true
		}
	}
	return Model{}, false
}

func fetch() ([]Model, error) {
	req, err := http.NewRequest(http.MethodGet, catalogURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	// nil account = global outbound settings (proxy/relay) apply.
	client := providers.GetRestClientForAccount(nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, providers.Errorf(resp.StatusCode, "models.dev HTTP %d", resp.StatusCode)
	}

	var raw rawCatalog
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&raw); err != nil {
		return nil, err
	}

	out := make([]Model, 0, 1024)
	for providerID, p := range raw {
		for modelID, m := range p.Models {
			id := m.ID
			if id == "" {
				id = modelID
			}
			out = append(out, Model{
				Provider:     providerID,
				ProviderName: p.Name,
				ID:           id,
				Name:         m.Name,
				Input:        m.Cost.Input,
				Output:       m.Cost.Output,
				CacheRead:    m.Cost.CacheRead,
				CacheWrite:   m.Cost.CacheWrite,
				Free:         m.Cost.Input == 0 && m.Cost.Output == 0,
				ContextLimit: m.Limit.Context,
				OutputLimit:  m.Limit.Output,
				Reasoning:    m.Reasoning,
				ToolCall:     m.ToolCall,
				Attachment:   m.Attachment,
				Modalities:   m.Modalities.Input,
				ReleaseDate:  m.ReleaseDate,
				Knowledge:    m.Knowledge,
			})
		}
	}
	return out, nil
}
