package clinepass

import (
	"encoding/json"
	"fmt"
	"hekato-go/auth"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"net/http"
	"strings"
	"time"
)

// init registers the admin import endpoint so the dashboard can POST a
// ClinePass token JSON to /auth/clinepass/import and have the account land in
// the pool. Mirrors providers/grok/routes.init + providers/codex/routes.init.
func init() {
	providers.RegisterAdminRoutes(map[string]providers.RouteHandler{
		"POST /auth/clinepass/import": importClinepass,
	})
}

// clinepassImportAccount is the per-account shape produced by ClinePass's
// browser OAuth flow (WorkOS JWT + refresh token) and by paste of a raw API
// key. The browser flow is wrapped by 9router's "clinepass" provider; a raw
// clp_… key (ClinePass consumer subscription) is accepted too.
//
// Either shape: `accessToken` (or workos-prefixed) is the bearer the proxy
// sends on the wire; `refreshToken` is optional and only required for
// OAuth-mode accounts that need rotation.
type clinepassImportAccount struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    string `json:"expires_at"`
	Email        string `json:"email"`
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
}

// importClinepass imports clinepass accounts from the OAuth-export shape.
// Accepts a single object, a JSON array, or newline-delimited JSON objects
// (bulk import). Each entry becomes an account with AuthMethod "clinepass".
func importClinepass(host providers.Host, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "read body failed: "+err.Error())
		return
	}

	entries, err := parseClinepassImportEntries(raw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(entries) == 0 {
		writeJSONError(w, http.StatusBadRequest, "no clinepass account entries found")
		return
	}

	imported := make([]map[string]interface{}, 0, len(entries))
	errors := make([]string, 0)
	for i, e := range entries {
		access := auth.NormalizeClinepassToken(e.AccessToken)
		if access == "" {
			errors = append(errors, fmt.Sprintf("entry %d: missing access_token", i))
			continue
		}
		email := strings.TrimSpace(e.Email)
		if email == "" {
			email = strings.TrimSpace(e.ID)
		}
		if email == "" {
			email = "clinepass account"
		}

		var expiresAt int64
		if e.ExpiresAt != "" {
			if t, perr := time.Parse(time.RFC3339Nano, e.ExpiresAt); perr == nil {
				expiresAt = t.Unix()
			}
		}
		if expiresAt == 0 {
			// WorkOS JWT path: derive exp from the embedded JWT claim if
			// present; fall back to 45-minute default. API-key paths land
			// here with ExpiresAt == 0 → the proxy will skip the refresh
			// dispatch (IsClinepassAPIKey).
			if e.ExpiresIn > 0 {
				expiresAt = time.Now().Unix() + int64(e.ExpiresIn)
			} else if exp := auth.ExpFromAccessTokenJWT(stripWorkOSPrefix(access)); exp > 0 {
				expiresAt = exp
			}
		}

		account := config.Account{
			ID:           auth.GenerateAccountID(),
			Email:        email,
			Nickname:     email,
			AccessToken:  access,
			RefreshToken: strings.TrimSpace(e.RefreshToken),
			AuthMethod:   "clinepass",
			Provider:     "clinepass",
			ProviderKind: "clinepass",
			ExpiresAt:    expiresAt,
			Enabled:      true,
			MachineId:    config.GenerateMachineId(),
		}
		if account.Email == "" {
			account.Email = "clinepass account"
		}

		if err := config.AddAccount(account); err != nil {
			errors = append(errors, fmt.Sprintf("entry %d: %v", i, err))
			continue
		}
		if err := host.RefreshAccountModels(&account); err != nil {
			logger.Warnf("[clinepass] model cache refresh failed for %s: %v", account.Email, err)
		}
		imported = append(imported, map[string]interface{}{
			"id":    account.ID,
			"email": account.Email,
		})
	}

	host.ReloadPool()

	if len(imported) == 0 && len(errors) > 0 {
		writeJSONError(w, http.StatusInternalServerError, strings.Join(errors, "; "))
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"imported": len(imported),
		"accounts": imported,
		"errors":   errors,
	})
}

// parseClinepassImportEntries accepts a single JSON object, a JSON array, or
// newline-delimited JSON objects (one per line, blank lines skipped). This
// mirrors the grok/codex importers so the admin dashboard can paste a 9router
// export as-is.
func parseClinepassImportEntries(raw []byte) ([]clinepassImportAccount, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("empty body")
	}

	// Try strict JSON first (single object OR array). This is the common 9router
	// export shape and lets the parser surface a useful "decode object" error
	// when the payload is genuinely malformed.
	if strings.HasPrefix(trimmed, "[") {
		var arr []clinepassImportAccount
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, fmt.Errorf("decode array: %w", err)
		}
		return arr, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		// Could be a single object OR multiple JSON objects on separate lines
		// (NDJSON paste). Try NDJSON first only when more than one '{' exists,
		// so the common single-object paste surfaces a clean "decode object"
		// error rather than a noisy "line 2: ..." one.
		if strings.Count(trimmed, "{") > 1 {
			lines := strings.Split(trimmed, "\n")
			var out []clinepassImportAccount
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var entry clinepassImportAccount
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					return nil, fmt.Errorf("decode ndjson line: %w", err)
				}
				out = append(out, entry)
			}
			if len(out) > 0 {
				return out, nil
			}
		}
		var single clinepassImportAccount
		if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
			return nil, fmt.Errorf("decode object: %w", err)
		}
		return []clinepassImportAccount{single}, nil
	}
	// Newline-delimited JSON with no leading brace (unusual).
	var out []clinepassImportAccount
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry clinepassImportAccount
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("decode ndjson line: %w", err)
		}
		out = append(out, entry)
	}
	return out, nil
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "success": "false"})
}

// stripWorkOSPrefix drops the literal "workos:" prefix from a ClinePass token
// so ExpFromAccessTokenJWT can decode the underlying JWT. Used when the
// operator pastes a workos-prefixed token and we still want to derive exp
// from the embedded claim.
func stripWorkOSPrefix(token string) string {
	t := strings.TrimSpace(token)
	if strings.HasPrefix(t, "workos:") {
		return strings.TrimPrefix(t, "workos:")
	}
	return t
}