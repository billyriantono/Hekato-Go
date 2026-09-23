package clinepass

import (
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"net/http"
	"strings"
)

// clinepassModelsURL is the live OpenAI-compatible catalog endpoint that
// ClinePass exposes. Returns an OpenAI-style {data: [...]} payload.
//
// We hit this at import time + on operator refresh; transient failures fall
// back to a static catalog so the admin model picker never goes blank.
func clinepassModelsURL() string { return clinepassBaseURL + "/api/v1/models" }

// clinepassStaticModels is the offline fallback catalog. Mirrors what the
// Cline IDE exposes today; updated here whenever the upstream gains or drops
// an alias so the picker stays useful while a live refresh is in flight.
//
// Reference: 9router open-sse/shared/models.js — entries prefixed "cline-pass/".
var clinepassStaticModels = []providers.ModelInfo{
	{ModelId: "cline-pass/claude-sonnet-4.6", ModelName: "Claude Sonnet 4.6 (ClinePass)", Description: "ClinePass pass-through to Anthropic Claude Sonnet 4.6."},
	{ModelId: "cline-pass/claude-sonnet-4.5", ModelName: "Claude Sonnet 4.5 (ClinePass)", Description: "ClinePass pass-through to Anthropic Claude Sonnet 4.5."},
	{ModelId: "cline-pass/claude-haiku-4.5", ModelName: "Claude Haiku 4.5 (ClinePass)", Description: "ClinePass pass-through to Anthropic Claude Haiku 4.5."},
	{ModelId: "cline-pass/gpt-5.4", ModelName: "GPT 5.4 (ClinePass)", Description: "ClinePass pass-through to OpenAI GPT 5.4."},
	{ModelId: "cline-pass/gpt-5.4-mini", ModelName: "GPT 5.4 Mini (ClinePass)", Description: "ClinePass pass-through to OpenAI GPT 5.4 Mini."},
	{ModelId: "cline-pass/gpt-5.3-codex", ModelName: "GPT 5.3 Codex (ClinePass)", Description: "ClinePass pass-through to OpenAI Codex."},
	{ModelId: "cline-pass/gemini-3-pro", ModelName: "Gemini 3 Pro (ClinePass)", Description: "ClinePass pass-through to Google Gemini 3 Pro."},
	{ModelId: "cline-pass/grok-4", ModelName: "Grok 4 (ClinePass)", Description: "ClinePass pass-through to xAI Grok 4."},
}

// clinepassModelEntry mirrors one element of the OpenAI /v1/models data array.
type clinepassModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// FetchModels lists the models the account can actually use, straight from
// ClinePass. Falls back to the static catalog when the live call fails so a
// transient upstream error never blanks the admin model picker.
func FetchModels(account *config.Account) ([]providers.ModelInfo, error) {
	if account == nil {
		return staticClinepassModels(), fmt.Errorf("clinepass: nil account")
	}
	if strings.TrimSpace(account.AccessToken) == "" {
		return staticClinepassModels(), fmt.Errorf("clinepass: account has no access token")
	}

	req, err := http.NewRequest(http.MethodGet, clinepassModelsURL(), nil)
	if err != nil {
		return staticClinepassModels(), fmt.Errorf("build clinepass models request: %w", err)
	}
	setClinepassHeaders(req, account)

	resp, err := providers.GetRestClientForAccount(account).Do(req)
	if err != nil {
		return staticClinepassModels(), fmt.Errorf("clinepass models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return staticClinepassModels(), providers.Errorf(resp.StatusCode, "clinepass models HTTP %d: %s", resp.StatusCode, raw)
	}

	var payload struct {
		Data []clinepassModelEntry `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return staticClinepassModels(), fmt.Errorf("decode clinepass models: %w", err)
	}

	models := make([]providers.ModelInfo, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		info := providers.ModelInfo{
			ModelId:    id,
			ModelName:  id,
			InputTypes: []string{"text"},
		}
		if m.OwnedBy != "" {
			info.Description = "ClinePass upstream: " + m.OwnedBy
		}
		models = append(models, info)
	}

	if len(models) == 0 {
		return staticClinepassModels(), fmt.Errorf("clinepass models: empty catalog")
	}
	logger.Infof("[clinepass] fetched %d live models for %s", len(models), account.Email)
	return models, nil
}

// staticClinepassModels returns a copy of the offline fallback catalog so
// callers can mutate the slice without affecting the canonical list.
func staticClinepassModels() []providers.ModelInfo {
	return append([]providers.ModelInfo(nil), clinepassStaticModels...)
}

// ModelsForAccount returns the ClinePass model list. Prefer the live
// per-account catalog; fall back to the static list when the account has no
// usable token (e.g. during import, before the first token refresh) or when
// the live call fails.
func ModelsForAccount(account *config.Account) []providers.ModelInfo {
	if account == nil || strings.TrimSpace(account.AccessToken) == "" {
		return staticClinepassModels()
	}
	models, err := FetchModels(account)
	if err != nil {
		logger.Warnf("[clinepass] live model fetch failed for %s, using static catalog: %v", account.Email, err)
	}
	return models
}

// RefreshModels is the admin-routes entry point. Fetches the live catalog
// and returns the slice; falls back to the static list when the live call
// fails (the caller is expected to persist the slice as-is).
func RefreshModels(account *config.Account) []providers.ModelInfo {
	if account == nil {
		return staticClinepassModels()
	}
	models, err := FetchModels(account)
	if err != nil {
		logger.Warnf("[clinepass] live model refresh failed for %s: %v", account.Email, err)
		return staticClinepassModels()
	}
	return models
}
