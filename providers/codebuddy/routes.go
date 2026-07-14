package codebuddy

import (
	"encoding/json"
	"kiro-go/auth"
	"kiro-go/config"
	"kiro-go/logger"
	"kiro-go/providers"
	"net/http"
	"strings"
)

func init() {
	providers.RegisterAdminRoutes(map[string]providers.RouteHandler{
		"POST /auth/codebuddy": importCodeBuddy,
	})
}

func importCodeBuddy(host providers.Host, w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey  string `json:"apiKey"`
		Label   string `json:"label"`
		Variant string `json:"variant"`
		Region  string `json:"region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "apiKey is required"})
		return
	}

	variant := strings.ToLower(strings.TrimSpace(req.Variant))
	region := strings.ToLower(strings.TrimSpace(req.Region))
	provider := "CodeBuddy"
	authMethod := "codebuddy"
	if variant == "cn" || variant == "china" || region == "cn" || strings.Contains(region, "china") {
		provider = "CodeBuddy CN"
		authMethod = "codebuddy-cn"
		region = "cn"
	}
	if region == "" {
		region = "global"
	}

	account := config.Account{
		ID:           auth.GenerateAccountID(),
		Email:        strings.TrimSpace(req.Label),
		Nickname:     strings.TrimSpace(req.Label),
		AccessToken:  apiKey,
		RefreshToken: apiKey,
		AuthMethod:   authMethod,
		Provider:     provider,
		Region:       region,
		Enabled:      true,
		MachineId:    config.GenerateMachineId(),
	}
	if account.Email == "" {
		account.Email = provider + " API Key"
	}

	if err := config.AddAccount(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	host.ReloadPool()
	if err := host.RefreshAccountModels(&account); err != nil {
		logger.Warnf("[CodeBuddy] Model cache refresh failed for %s: %v", account.Email, err)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"account": map[string]interface{}{
			"id":         account.ID,
			"email":      account.Email,
			"authMethod": account.AuthMethod,
			"provider":   account.Provider,
		},
	})
}
