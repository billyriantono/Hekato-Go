package codebuddy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
)

// ModelsForAccount returns the CLI's bundled picker for global accounts and
// refreshes the China catalog from /v3/config. Always live: the operator's
// model-refresh buttons and tests must see the upstream truth. The background
// warmup loop uses CachedModelsForAccount instead, so a pool of same-origin
// accounts does not poll /v3/config in lockstep every cycle — synchronized
// polling from one egress IP is the pattern that gets accounts flagged as
// automated (code 11140). A failed metadata request must not disable an
// otherwise usable inference account.
func ModelsForAccount(account *config.Account) []providers.ModelInfo {
	if variantForAccount(account).Name == codeBuddyGlobal.Name || codeBuddyToken(account) == "" {
		return staticModelsForAccount(account)
	}
	models, err := FetchModels(account)
	if err != nil {
		logger.Warnf("[CodeBuddy] %s live model discovery failed, using offline catalog: %v", variantForAccount(account).Name, err)
		return staticModelsForAccount(account)
	}
	storeModelsCache(account, models)
	return appendCustomModels(models)
}

// CachedModelsForAccount is the warmup-loop entry point: same catalog, but
// /v3/config is hit at most once per catalog TTL per variant. Every successful
// live fetch (via ModelsForAccount) refreshes the cache, so an operator refresh
// also warms it.
func CachedModelsForAccount(account *config.Account) []providers.ModelInfo {
	if variantForAccount(account).Name == codeBuddyGlobal.Name || codeBuddyToken(account) == "" {
		return staticModelsForAccount(account)
	}
	if models, ok := cachedModelsFor(account); ok {
		return appendCustomModels(models)
	}
	return ModelsForAccount(account)
}

// catalogTTL: how long a fetched /v3/config catalog stays fresh. Model lists
// change on CLI releases, not hourly.
const catalogTTL = 6 * time.Hour

type catalogEntry struct {
	models  []providers.ModelInfo
	fetched time.Time
}

var (
	catalogMu    sync.Mutex
	catalogCache = map[string]catalogEntry{} // account ID → cache
)

// cachedModelsFor returns a fresh cached catalog for the account. Keyed by
// account ID, not variant: entitlements (image support, model set) differ per
// token, and collapsing them would corrupt one account's catalog with
// another's.
func cachedModelsFor(account *config.Account) ([]providers.ModelInfo, bool) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	entry, ok := catalogCache[account.ID]
	if !ok || time.Since(entry.fetched) > catalogTTL {
		return nil, false
	}
	return entry.models, true
}

func storeModelsCache(account *config.Account, models []providers.ModelInfo) {
	catalogMu.Lock()
	catalogCache[account.ID] = catalogEntry{models: models, fetched: time.Now()}
	catalogMu.Unlock()
}

// InvalidateModelCatalog forgets cached /v3/config catalogs so the next
// CachedModelsForAccount refetches (tests, forced refreshes).
func InvalidateModelCatalog() {
	catalogMu.Lock()
	catalogCache = map[string]catalogEntry{}
	catalogMu.Unlock()
}

// FetchModels reads the product configuration endpoint used by the official
// CLI, not the unsupported OpenAI /models endpoint. No repository information
// is sent. Only catalog metadata is decoded from the configuration document.
func FetchModels(account *config.Account) ([]providers.ModelInfo, error) {
	token := codeBuddyToken(account)
	if token == "" {
		return nil, fmt.Errorf("codebuddy models: missing credential")
	}
	v := variantForAccount(account)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.Base+"/v3/config", nil)
	if err != nil {
		return nil, fmt.Errorf("codebuddy models request: %w", err)
	}
	applyCodeBuddyHeaders(req.Header, v, token)
	if userID := strings.TrimSpace(account.UserId); userID != "" {
		req.Header.Set("X-User-Id", userID)
	}
	resp, err := providers.GetRestClientForAccount(account).Do(req)
	if err != nil {
		return nil, fmt.Errorf("codebuddy models request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codebuddy models: HTTP %d", resp.StatusCode)
	}
	const maxCatalogBytes = 16 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read codebuddy models: %w", err)
	}
	if len(body) > maxCatalogBytes {
		return nil, fmt.Errorf("codebuddy models: configuration exceeds size limit")
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Models []struct {
				ID                 string `json:"id"`
				Name               string `json:"name"`
				DescriptionEn      string `json:"descriptionEn"`
				DescriptionZh      string `json:"descriptionZh"`
				SupportsImages     bool   `json:"supportsImages"`
				DisabledMultimodal bool   `json:"disabledMultimodal"`
				MaxInputTokens     int    `json:"maxInputTokens"`
				MaxOutputTokens    int    `json:"maxOutputTokens"`
			} `json:"models"`
			Agents []struct {
				Name   string   `json:"name"`
				Models []string `json:"models"`
			} `json:"agents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode codebuddy models: %w", err)
	}
	if payload.Code != 0 {
		return nil, fmt.Errorf("codebuddy models: upstream code %d", payload.Code)
	}
	models := make([]providers.ModelInfo, 0, len(payload.Data.Models))
	seen := make(map[string]bool, len(payload.Data.Models))
	for _, entry := range payload.Data.Models {
		id := strings.TrimSpace(entry.ID)
		key := strings.ToLower(id)
		if id == "" || seen[key] {
			continue
		}
		seen[key] = true
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = id
		}
		description := entry.DescriptionEn
		if description == "" {
			description = entry.DescriptionZh
		}
		model := providers.ModelInfo{ModelId: id, ModelName: name, Description: description, InputTypes: []string{"text"}}
		if entry.SupportsImages && !entry.DisabledMultimodal {
			model.InputTypes = append(model.InputTypes, "image")
		}
		if entry.MaxInputTokens > 0 || entry.MaxOutputTokens > 0 {
			model.TokenLimits = &struct {
				MaxInputTokens  int `json:"maxInputTokens"`
				MaxOutputTokens int `json:"maxOutputTokens"`
			}{MaxInputTokens: entry.MaxInputTokens, MaxOutputTokens: entry.MaxOutputTokens}
		}
		models = append(models, model)
	}
	for _, agent := range payload.Data.Agents {
		if agent.Name == "cli" {
			models = cliModels(models, agent.Models)
			break
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("codebuddy models: empty catalog")
	}
	logger.Infof("[CodeBuddy] Fetched %d live models from %s", len(models), v.Name)
	return models, nil
}

// cliModels narrows the full product catalog to the official CLI's picker:
// data.models also lists internal models (next-edit, completion, image/video
// generation) that the CLI never offers. The CLI order is kept; if no allowed
// id is in the catalog, the whole catalog is kept.
func cliModels(models []providers.ModelInfo, allow []string) []providers.ModelInfo {
	byID := make(map[string]providers.ModelInfo, len(models))
	for _, model := range models {
		byID[strings.ToLower(model.ModelId)] = model
	}
	out := make([]providers.ModelInfo, 0, len(allow))
	for _, id := range allow {
		key := strings.ToLower(strings.TrimSpace(id))
		if model, ok := byID[key]; ok {
			out = append(out, model)
			delete(byID, key)
		}
	}
	if len(out) == 0 {
		return models
	}
	return out
}

func appendCustomModels(models []providers.ModelInfo) []providers.ModelInfo {
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		seen[strings.ToLower(model.ModelId)] = true
	}
	for _, id := range config.GetCustomModelIDs("codebuddy") {
		id = strings.TrimSpace(id)
		key := strings.ToLower(id)
		if id == "" || seen[key] {
			continue
		}
		seen[key] = true
		models = append(models, providers.ModelInfo{ModelId: id, ModelName: id, InputTypes: []string{"text"}})
	}
	return models
}
