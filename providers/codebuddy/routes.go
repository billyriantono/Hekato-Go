package codebuddy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hekato-go/auth"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"net/http"
	"strings"
)

func init() {
	providers.RegisterAdminRoutes(map[string]providers.RouteHandler{
		"POST /auth/codebuddy": importCodeBuddy,
	})
}

// isCodeBuddyCNToken reports whether a pasted CodeBuddy credential belongs to the
// China gateway, by inspecting the issuer of the token's (unverified) JWT
// payload. CN tokens are issued by the Keycloak realm on codebuddy.cn (or, for
// older exports, copilot.tencent.com); global tokens carry codebuddy.ai.
// Non-JWT credentials (plain ck_* API keys) return false — the operator's
// explicit variant then decides.
func isCodeBuddyCNToken(token string) bool {
	iss := strings.ToLower(auth.IssuerFromAccessTokenJWT(token))
	if iss == "" {
		return false
	}
	return strings.Contains(iss, "codebuddy.cn") || strings.Contains(iss, "copilot.tencent.com")
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

	creds := parseCodeBuddyCredentials(req.APIKey)
	var tokens []string
	for _, c := range creds {
		tokens = append(tokens, c.Access)
	}
	if len(tokens) == 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "apiKey is required (one API key or JWT session token per line)"})
		return
	}

	baseVariant := strings.ToLower(strings.TrimSpace(req.Variant))
	baseRegion := strings.ToLower(strings.TrimSpace(req.Region))
	forceCN := baseVariant == "cn" || baseVariant == "china" || baseRegion == "cn" || strings.Contains(baseRegion, "china")

	label := strings.TrimSpace(req.Label)
	var imported []map[string]interface{}
	var errs []string
	for i, c := range creds {
		tok := c.Access
		// A CodeBuddy China credential is only valid against the CN gateway; sent
		// to the global endpoint it answers 401 from the APISIX edge. CN and
		// global tokens are otherwise indistinguishable (both are Keycloak JWTs),
		// so when the operator doesn't say which region this is, recover it from
		// the token's own issuer rather than defaulting to global and silently
		// producing a dead account. Detection is per token so a mixed batch (CN +
		// global) lands each credential on the right gateway.
		provider, authMethod, region := "CodeBuddy", "codebuddy", baseRegion
		if forceCN || isCodeBuddyCNToken(tok) {
			provider, authMethod, region = "CodeBuddy CN", "codebuddy-cn", "cn"
		} else if region == "" {
			region = "global"
		}

		name := label
		if name == "" {
			name = c.UID
		}
		if name == "" {
			name = provider + " " + credentialKind(tok)
		}
		if len(creds) > 1 && (label != "" || c.UID == "") {
			name = fmt.Sprintf("%s #%d", name, i+1)
		}
		// Token JSON carries a Keycloak offline refresh token; bare credentials
		// store the access token twice (no refresh flow).
		refresh := c.Refresh
		if refresh == "" {
			refresh = tok
		}
		account := config.Account{
			ID:           auth.GenerateAccountID(),
			Email:        name,
			Nickname:     name,
			UserId:       c.UID,
			AccessToken:  tok,
			RefreshToken: refresh,
			AuthMethod:   authMethod,
			Provider:     provider,
			Region:       region,
			ExpiresAt:    jwtExpiry(tok),
			Enabled:      true,
			MachineId:    config.GenerateMachineId(),
		}
		if err := config.AddAccount(account); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		host.ReloadPool()
		if err := host.RefreshAccountModels(&account); err != nil {
			logger.Warnf("[CodeBuddy] Model cache refresh failed for %s: %v", account.Email, err)
		}
		imported = append(imported, map[string]interface{}{
			"id": account.ID, "email": account.Email, "authMethod": account.AuthMethod, "provider": account.Provider,
			"kind": credentialKind(tok), "expiresAt": account.ExpiresAt, "refreshable": auth.CodeBuddyRefreshable(&account),
		})
	}
	if len(imported) == 0 {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": strings.Join(errs, "; ")})
		return
	}
	first := imported[0]
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"account":  first,
		"accounts": imported,
		"errors":   errs,
	})
}

// codeBuddyCredential is one parsed credential: an API key, a bare session
// token, or a token JSON ({"access_token","refresh_token","uid"}).
type codeBuddyCredential struct {
	Access  string
	Refresh string
	UID     string
}

// parseCodeBuddyCredentials accepts, in one text blob: a JSON object, a JSON
// array of objects, one JSON object per line, and/or bare keys / tokens one
// per line. Objects may use access_token/refresh_token/uid or camelCase names.
func parseCodeBuddyCredentials(text string) []codeBuddyCredential {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []codeBuddyCredential
	addObj := func(m map[string]interface{}) bool {
		get := func(keys ...string) string {
			for _, k := range keys {
				if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
			return ""
		}
		access := get("access_token", "accessToken", "token", "api_key", "apiKey")
		if access == "" {
			return false
		}
		out = append(out, codeBuddyCredential{
			Access:  access,
			Refresh: get("refresh_token", "refreshToken"),
			UID:     get("uid", "user_id", "userId", "email", "username"),
		})
		return true
	}
	// Whole blob as one JSON document (pretty-printed object or array).
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		var one map[string]interface{}
		if json.Unmarshal([]byte(text), &one) == nil {
			if addObj(one) {
				return out
			}
		}
		var many []map[string]interface{}
		if json.Unmarshal([]byte(text), &many) == nil {
			for _, m := range many {
				addObj(m)
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	// Line by line: JSON objects or bare credentials.
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") {
			var m map[string]interface{}
			if json.Unmarshal([]byte(line), &m) == nil && addObj(m) {
				continue
			}
			continue // unparsable JSON line: skip rather than store garbage
		}
		out = append(out, codeBuddyCredential{Access: line})
	}
	return out
}

// credentialKind labels a CodeBuddy credential for display.
func credentialKind(tok string) string {
	if isJWT(tok) {
		return "session token"
	}
	return "API key"
}

// jwtExpiry returns the exp claim of a JWT (unix seconds) or 0.
func jwtExpiry(tok string) int64 {
	if !isJWT(tok) {
		return 0
	}
	parts := strings.Split(tok, ".")
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return 0
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return 0
	}
	return claims.Exp
}
