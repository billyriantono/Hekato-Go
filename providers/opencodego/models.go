package opencodego

import (
	"encoding/json"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"net/http"
	"time"
)

// goStaticModels is the fallback catalog for OpenCode Go subscription tier
// ($5/mo, then $10/mo). Mirrors 9router's opencode-go registry.
var goStaticModels = []providers.ModelInfo{
	{ModelId: "claude-sonnet-4-20250514"},
	{ModelId: "gpt-4.1"},
	{ModelId: "gemini-2.5-pro"},
	{ModelId: "deepseek-v4-flash"},
	{ModelId: "grok-4.6"},
	{ModelId: "gpt-5.6-luna"},
	{ModelId: "kimi-k2"},
	{ModelId: "qwen3-coder"},
	{ModelId: "muse-spark-1.2-contributor"},
	{ModelId: "muse-spark-1.3-contributor"},
}

// RefreshModels fetches the live model list from the OpenCode Go /models
// endpoint, falling back to the static catalog on any failure.
func RefreshModels(account *config.Account) []providers.ModelInfo {
	models, err := fetchModels(account)
	if err != nil || len(models) == 0 {
		if err != nil {
			logger.Warnf("[opencodego] model refresh failed: %v; using static list", err)
		}
		return append([]providers.ModelInfo{}, goStaticModels...)
	}
	return models
}

// ModelsForAccount returns the model catalog for the admin panel.
func ModelsForAccount(account *config.Account) []providers.ModelInfo {
	return RefreshModels(account)
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func fetchModels(account *config.Account) ([]providers.ModelInfo, error) {
	req, err := http.NewRequest(http.MethodGet, goModelsURL, nil)
	if err != nil {
		return nil, err
	}
	token := account.AccessToken
	if token == "" {
		token = "public"
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", opencodeUA)
	req.Header.Set("x-opencode-client", "desktop")
	req.Header.Set("Accept", "application/json")

	client := providers.GetRestClientForAccount(account)
	client.Timeout = 5 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, providers.Errorf(resp.StatusCode, "opencodego models HTTP %d", resp.StatusCode)
	}

	var body modelsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]providers.ModelInfo, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID == "" {
			continue
		}
		out = append(out, providers.ModelInfo{ModelId: m.ID})
	}
	return out, nil
}