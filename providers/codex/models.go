package codex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
)

// codexModelsURL is the live Codex model catalog. It is account-scoped: the
// returned set depends on the account's plan (free accounts see a subset), so
// the list must be fetched per account rather than hardcoded. `client_version`
// is mandatory — without it the endpoint answers 400 with a validation error.
const codexModelsURL = "https://chatgpt.com/backend-api/codex/models"

// codexStaticModels is the offline fallback. Used only when the live catalog
// can't be reached (network failure, expired token, pre-refresh import), so the
// dashboard still shows something actionable instead of an empty list. Kept in
// sync with 9router's registry catalog.
var codexStaticModels = []providers.ModelInfo{
	{ModelId: "gpt-5.6-sol", ModelName: "GPT 5.6 Sol"},
	{ModelId: "gpt-5.6-terra", ModelName: "GPT 5.6 Terra"},
	{ModelId: "gpt-5.6-luna", ModelName: "GPT 5.6 Luna"},
	{ModelId: "gpt-5.5", ModelName: "GPT 5.5"},
	{ModelId: "gpt-5.4", ModelName: "GPT 5.4"},
	{ModelId: "gpt-5.4-mini", ModelName: "GPT 5.4 Mini"},
	{ModelId: "gpt-5.3-codex-spark", ModelName: "GPT 5.3 Codex Spark"},
	{ModelId: "gpt-5-codex", ModelName: "GPT 5 Codex"},
	{ModelId: "gpt-5", ModelName: "GPT 5"},
	{ModelId: "codex-auto-review", ModelName: "Codex Auto Review"},
}

// codexModelEntry is one entry of the live catalog. Only the fields the
// dashboard and router need are decoded; the upstream payload carries ~40 keys
// per model including large instruction blobs we deliberately ignore.
type codexModelEntry struct {
	Slug             string   `json:"slug"`
	DisplayName      string   `json:"display_name"`
	Description      string   `json:"description"`
	InputModalities  []string `json:"input_modalities"`
	ContextWindow    int      `json:"context_window"`
	MaxContextWindow int      `json:"max_context_window"`
	MinimalClientVer string   `json:"minimal_client_version"`
	AvailableInPlans []string `json:"available_in_plans"`
}

// FetchModels lists the models the account can actually use, straight from the
// Codex backend. Falls back to the static catalog when the live call fails so a
// transient upstream error never blanks the admin model picker.
func FetchModels(account *config.Account) ([]providers.ModelInfo, error) {
	if account == nil {
		return staticModels(), fmt.Errorf("codex: nil account")
	}

	req, err := http.NewRequest(http.MethodGet, codexModelsURL+"?client_version="+url.QueryEscape(codexClientVer), nil)
	if err != nil {
		return staticModels(), fmt.Errorf("build codex models request: %w", err)
	}
	setCodexHeaders(req, account)

	resp, err := providers.GetRestClientForAccount(account).Do(req)
	if err != nil {
		return staticModels(), fmt.Errorf("codex models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return staticModels(), providers.Errorf(resp.StatusCode, "codex models HTTP %d: %s", resp.StatusCode, raw)
	}

	var payload struct {
		Models []codexModelEntry `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return staticModels(), fmt.Errorf("decode codex models: %w", err)
	}

	models := make([]providers.ModelInfo, 0, len(payload.Models))
	for _, m := range payload.Models {
		slug := strings.TrimSpace(m.Slug)
		if slug == "" {
			continue
		}
		info := providers.ModelInfo{
			ModelId:     slug,
			ModelName:   m.DisplayName,
			Description: m.Description,
			InputTypes:  m.InputModalities,
		}
		if info.ModelName == "" {
			info.ModelName = slug
		}
		if info.InputTypes == nil {
			info.InputTypes = []string{"text"}
		}
		maxCtx := m.MaxContextWindow
		if maxCtx == 0 {
			maxCtx = m.ContextWindow
		}
		if maxCtx > 0 {
			info.TokenLimits = &struct {
				MaxInputTokens  int `json:"maxInputTokens"`
				MaxOutputTokens int `json:"maxOutputTokens"`
			}{MaxInputTokens: maxCtx}
		}
		models = append(models, info)
	}

	if len(models) == 0 {
		return staticModels(), fmt.Errorf("codex models: empty catalog")
	}
	logger.Infof("[codex] fetched %d live models for %s", len(models), account.Email)
	return models, nil
}

// staticModels returns a copy of the offline fallback catalog — callers may
// mutate the slice without affecting the canonical list.
func staticModels() []providers.ModelInfo {
	return append([]providers.ModelInfo(nil), codexStaticModels...)
}

// ModelsForAccount returns the Codex model list. Prefer the live per-account
// catalog; fall back to the static list when the account has no usable token
// (e.g. during import, before the first token refresh).
func ModelsForAccount(account *config.Account) []providers.ModelInfo {
	if account == nil || strings.TrimSpace(account.AccessToken) == "" {
		return staticModels()
	}
	models, err := FetchModels(account)
	if err != nil {
		logger.Warnf("[codex] live model fetch failed for %s, using static catalog: %v", account.Email, err)
	}
	return models
}
