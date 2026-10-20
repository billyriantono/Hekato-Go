package codebuddy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
)

// ModelsForAccount refreshes the account's regional catalog on every call.
// The proxy owns scheduling, pool routing caches and persistence. A failed
// metadata request must not disable an otherwise usable inference account.
func ModelsForAccount(account *config.Account) []providers.ModelInfo {
	if codeBuddyToken(account) == "" {
		return staticModelsForAccount(account)
	}
	models, err := FetchModels(account)
	if err != nil {
		logger.Warnf("[CodeBuddy] %s live model discovery failed, using offline catalog: %v", variantForAccount(account).Name, err)
		return staticModelsForAccount(account)
	}
	return appendCustomModels(models)
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
	if len(models) == 0 {
		return nil, fmt.Errorf("codebuddy models: empty catalog")
	}
	logger.Infof("[CodeBuddy] Fetched %d live models from %s", len(models), v.Name)
	return models, nil
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
