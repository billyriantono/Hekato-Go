package anthropiccompat

import (
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/providers"
	"io"
	"net/http"
	"strings"
	"time"
)

// anthropicModelsResponse mirrors the Anthropic /v1/models response.
type anthropicModelsResponse struct {
	Data []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"data"`
}

// ListModels fetches the model catalog from the configured vendor's
// /v1/models endpoint. Returns an empty slice on any failure so the caller
// can fall back to the account's ExtraModels.
func ListModels(account *config.Account) ([]providers.ModelInfo, error) {
	if account == nil || account.BaseURL == "" {
		return nil, fmt.Errorf("anthropiccompat: account %s missing base URL", accountID(account))
	}
	baseURL = account.BaseURL
	req, err := http.NewRequest(http.MethodGet, modelsURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("build models request: %w", err)
	}
	if account.CompatAPIKey != "" {
		req.Header.Set("x-api-key", account.CompatAPIKey)
		req.Header.Set("Authorization", "Bearer "+account.CompatAPIKey)
	}
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("Accept", "application/json")

	client := providers.GetRestClientForAccount(account)
	client.Timeout = 5 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropiccompat models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("anthropiccompat models HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var body anthropicModelsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode anthropiccompat models: %w", err)
	}

	out := make([]providers.ModelInfo, 0, len(body.Data))
	for _, m := range body.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		out = append(out, providers.ModelInfo{ModelId: id})
	}
	return out, nil
}

// ModelsForAccount returns the live /v1/models catalog plus any
// operator-added ExtraModels. ExtraModels that are already in the catalog
// are deduped so the picker never shows duplicates.
func ModelsForAccount(account *config.Account) []providers.ModelInfo {
	seen := map[string]struct{}{}
	out := []providers.ModelInfo{}
	live, err := ListModels(account)
	if err == nil {
		for _, m := range live {
			if _, dup := seen[m.ModelId]; dup {
				continue
			}
			seen[m.ModelId] = struct{}{}
			out = append(out, m)
		}
	}
	for _, id := range account.ExtraModels {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, providers.ModelInfo{ModelId: id})
	}
	return out
}

func accountID(a *config.Account) string {
	if a == nil {
		return "<nil>"
	}
	return a.ID
}