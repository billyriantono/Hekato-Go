package opencodezen

import (
	"encoding/json"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"net/http"
	"time"
)

// zenStaticModels is the fallback catalog when the live /models endpoint is
// unreachable. Jev is absent: its SystemOne decision protocol is not a chat
// or Responses API, so exposing it here would create unroutable requests.
var zenStaticModels = []providers.ModelInfo{
	{ModelId: "big-pickle"},
	{ModelId: "ling-3.0-flash-fin-free"},
	{ModelId: "mimo-v2.5-free"},
	{ModelId: "mimo-v2.6-flash-free"},
	{ModelId: "muse-spark-1.2-contributor-free"},
	{ModelId: "muse-spark-1.3-contributor-free"},
	{ModelId: "nemotron-3-ultra-free"},
	{ModelId: "nemotron-3.5-lightning-free"},
}

// RefreshModels fetches the live model list from the Zen /models endpoint,
// falling back to the static catalog on any failure.
func RefreshModels(account *config.Account) []providers.ModelInfo {
	models, err := fetchModels(account)
	if err != nil || len(models) == 0 {
		if err != nil {
			logger.Warnf("[opencodezen] model refresh failed: %v; using static list", err)
		}
		return append([]providers.ModelInfo{}, zenStaticModels...)
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
	req, err := http.NewRequest(http.MethodGet, zenModelsURL, nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req, account)
	req.Header.Set("Accept", "application/json")

	client := providers.GetRestClientForAccount(account)
	client.Timeout = 5 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, providers.Errorf(resp.StatusCode, "opencodezen models HTTP %d", resp.StatusCode)
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
