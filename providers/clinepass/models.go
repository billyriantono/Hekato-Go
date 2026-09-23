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
	// Verified 2026-09-23 by probing api.cline.bot with a ClinePass account:
	// these IDs are accepted by the pass (they answer the monthly-cap 429
	// rather than "model not found"). The live /models catalog never lists them.
	{ModelId: "cline-pass/kimi-k3", ModelName: "Kimi K3 (ClinePass)"},
	{ModelId: "cline-pass/minimax-m3", ModelName: "MiniMax M3 (ClinePass)"},
	{ModelId: "cline-pass/glm-5.3", ModelName: "GLM 5.3 (ClinePass)"},
	{ModelId: "cline-pass/glm-5.3-flash", ModelName: "GLM 5.3 Flash (ClinePass)"},
	{ModelId: "cline-pass/deepseek-v4-pro", ModelName: "DeepSeek V4 Pro (ClinePass)"},
	{ModelId: "cline-pass/deepseek-v4.1-flash", ModelName: "DeepSeek V4.1 Flash (ClinePass)"},
	{ModelId: "cline-pass/mimo-v2.5-pro", ModelName: "MiMo V2.5 Pro (ClinePass)"},
	{ModelId: "cline-pass/mimo-v2.5", ModelName: "MiMo V2.5 (ClinePass)"},
	{ModelId: "cline-pass/qwen3.7-max", ModelName: "Qwen 3.7 Max (ClinePass)"},
	{ModelId: "cline-pass/qwen3.7-plus", ModelName: "Qwen 3.7 Plus (ClinePass)"},
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
