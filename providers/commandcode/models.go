package commandcode

import (
	"encoding/json"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"net/http"
	"sort"
	"strings"
)

// commandCodeStaticModels is the fallback catalog used when the live
// /models endpoint is unreachable. CommandCode does not publish a public
// model list, so this mirrors the IDs its chat UI exposes.
var commandCodeStaticModels = []providers.ModelInfo{
	{ModelId: "deepseek/deepseek-v4-pro"},
	{ModelId: "deepseek/deepseek-v4-flash"},
	{ModelId: "moonshotai/Kimi-K2.7-Code"},
	{ModelId: "moonshotai/Kimi-K2.7-Code-Highspeed"},
	{ModelId: "moonshotai/Kimi-K2.6"},
	{ModelId: "moonshotai/Kimi-K2.5"},
	{ModelId: "zai-org/GLM-5.2"},
	{ModelId: "zai-org/GLM-5.2-Fast"},
	{ModelId: "zai-org/GLM-5.1"},
	{ModelId: "zai-org/GLM-5"},
	{ModelId: "MiniMaxAI/MiniMax-M3"},
	{ModelId: "MiniMaxAI/MiniMax-M2.7"},
	{ModelId: "MiniMaxAI/MiniMax-M2.5"},
	{ModelId: "xiaomi/mimo-v2.5-pro"},
	{ModelId: "xiaomi/mimo-v2.5"},
	{ModelId: "Qwen/Qwen3.6-Max-Preview"},
	{ModelId: "Qwen/Qwen3.6-Plus"},
	{ModelId: "Qwen/Qwen3.7-Max"},
	{ModelId: "Qwen/Qwen3.7-Plus"},
	{ModelId: "stepfun/Step-3.7-Flash"},
	{ModelId: "stepfun/Step-3.5-Flash"},
	{ModelId: "nvidia/nemotron-3-ultra-550b-a55b"},
}

// ModelsForAccount returns the catalog used by the admin panel. The upstream
// does not expose a per-account model list, so we always return the static
// catalog.
func ModelsForAccount(_ *config.Account) []providers.ModelInfo {
	out := make([]providers.ModelInfo, len(commandCodeStaticModels))
	copy(out, commandCodeStaticModels)
	return out
}

// RefreshModels keeps the adapter wiring symmetric with other providers
// (OpenCode Zen, OpenCode Go). Live /models isn't available upstream, so the
// static catalog is returned unchanged.
func RefreshModels(account *config.Account) []providers.ModelInfo {
	models, err := fetchLiveModels(account)
	if err != nil || len(models) == 0 {
		if err != nil {
			logger.Debugf("[CommandCode] live model fetch failed: %v; using static list", err)
		}
		return ModelsForAccount(account)
	}
	return models
}

type ccModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func fetchLiveModels(account *config.Account) ([]providers.ModelInfo, error) {
	if account == nil || strings.TrimSpace(account.AccessToken) == "" {
		return nil, nil
	}
	req, err := http.NewRequest(http.MethodGet, upstreamURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req, account, false)
	req.Header.Set("Accept", "application/json")

	client := providers.GetRestClientForAccount(account)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, providers.Errorf(resp.StatusCode, "commandcode models HTTP %d", resp.StatusCode)
	}
	var body ccModelsResponse
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
	sort.Slice(out, func(i, j int) bool { return out[i].ModelId < out[j].ModelId })
	return out, nil
}
