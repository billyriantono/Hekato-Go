package grok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kiro-go/auth"
	"kiro-go/config"
	"kiro-go/logger"
	"kiro-go/providers"
	"net/http"
	"strings"
	"time"
)

func init() {
	providers.RegisterAdminRoutes(map[string]providers.RouteHandler{
		"POST /auth/grok/start":  startGrokAuth,
		"POST /auth/grok/poll":   pollGrokAuth,
		"POST /auth/grok/import": importGrok,
	})
}

// apiStartGrokAuth begins a Grok (xAI) device-code login and returns the
// user_code / verification URI for the operator to enter at x.ai/activate.
func startGrokAuth(host providers.Host, w http.ResponseWriter, r *http.Request) {
	session, err := auth.StartGrokLogin()
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId":       session.ID,
		"userCode":        session.UserCode,
		"verificationUri": session.VerificationURI,
		"interval":        session.Interval,
	})
}

// apiPollGrokAuth polls the device-code login. While pending it returns
// completed=false; once authorized it persists a grok account and returns
// completed=true.
func pollGrokAuth(host providers.Host, w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	accessToken, refreshToken, expiresIn, status, err := auth.PollGrokAuth(req.SessionID)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if status == "pending" || status == "slow_down" {
		interval := 5
		if session := auth.GetGrokSession(req.SessionID); session != nil {
			interval = session.Interval
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"completed": false,
			"status":    status,
			"interval":  interval,
		})
		return
	}

	// Authorization complete. Recover the id_token (not returned by PollGrokAuth)
	// by decoding it separately is not possible here; email is best-effort via the
	// user endpoint. We register the account and resolve email lazily on first use.
	email, _, _ := auth.GetGrokUserInfo(accessToken, "")

	account := config.Account{
		ID:           auth.GenerateAccountID(),
		Email:        email,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		AuthMethod:   "grok",
		Provider:     "grok",
		ExpiresAt:    time.Now().Unix() + int64(expiresIn),
		Enabled:      true,
		MachineId:    config.GenerateMachineId(),
	}

	if err := config.AddAccount(account); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	host.ReloadPool()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"completed": true,
		"account": map[string]interface{}{
			"id":    account.ID,
			"email": account.Email,
		},
	})
}

// grokImportAccount is one entry of the grok token export format:
// { email, password, tokens: { access_token, refresh_token, expires_at,
//
//	expires_in, email, client_id, id_token }, smoke: {...} }
type grokImportAccount struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Tokens   struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    string `json:"expires_at"`
		ExpiresIn    int    `json:"expires_in"`
		Email        string `json:"email"`
		ClientID     string `json:"client_id"`
		IDToken      string `json:"id_token"`
	} `json:"tokens"`
}

// apiImportGrok imports grok accounts from the grok token export format.
// Accepts a single object, a JSON array, or newline-delimited JSON objects
// (bulk import). Each entry becomes an account with AuthMethod "grok".
func importGrok(host providers.Host, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "read body failed: " + err.Error()})
		return
	}

	entries, err := parseGrokImportEntries(raw)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if len(entries) == 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "no grok account entries found"})
		return
	}

	imported := make([]map[string]interface{}, 0, len(entries))
	errors := make([]string, 0)
	for i, e := range entries {
		access := strings.TrimSpace(e.Tokens.AccessToken)
		if access == "" {
			errors = append(errors, fmt.Sprintf("entry %d: missing access_token", i))
			continue
		}
		email := strings.TrimSpace(e.Tokens.Email)
		if email == "" {
			email = strings.TrimSpace(e.Email)
		}

		var expiresAt int64
		if e.Tokens.ExpiresAt != "" {
			if t, perr := time.Parse(time.RFC3339Nano, e.Tokens.ExpiresAt); perr == nil {
				expiresAt = t.Unix()
			}
		}
		if expiresAt == 0 {
			if e.Tokens.ExpiresIn > 0 {
				expiresAt = time.Now().Unix() + int64(e.Tokens.ExpiresIn)
			} else {
				expiresAt = auth.ExpFromAccessTokenJWT(access)
			}
		}

		account := config.Account{
			ID:           auth.GenerateAccountID(),
			Email:        email,
			Nickname:     email,
			AccessToken:  access,
			RefreshToken: strings.TrimSpace(e.Tokens.RefreshToken),
			AuthMethod:   "grok",
			Provider:     "grok",
			ExpiresAt:    expiresAt,
			Enabled:      true,
			MachineId:    config.GenerateMachineId(),
		}
		if account.Email == "" {
			account.Email = "grok account"
		}

		if err := config.AddAccount(account); err != nil {
			errors = append(errors, fmt.Sprintf("entry %d: %v", i, err))
			continue
		}
		if err := host.RefreshAccountModels(&account); err != nil {
			logger.Warnf("[grok] model cache refresh failed for %s: %v", account.Email, err)
		}
		imported = append(imported, map[string]interface{}{
			"id":    account.ID,
			"email": account.Email,
		})
	}

	host.ReloadPool()

	if len(imported) == 0 && len(errors) > 0 {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   strings.Join(errors, "; "),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"imported": len(imported),
		"accounts": imported,
		"errors":   errors,
	})
}

// parseGrokImportEntries accepts a single JSON object, a JSON array, or
// newline-delimited JSON objects (one per line, blank lines skipped).
func parseGrokImportEntries(raw []byte) ([]grokImportAccount, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty body")
	}
	// Array form: [ {...}, {...} ]
	if trimmed[0] == '[' {
		var arr []grokImportAccount
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, fmt.Errorf("parse array: %w", err)
		}
		return arr, nil
	}
	// Everything else (single object or newline-delimited JSON) is parsed line-by-line.
	// A bare single object is just a one-line "NDJSON" input.
	var out []grokImportAccount
	for _, line := range bytes.Split(trimmed, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var one grokImportAccount
		if err := json.Unmarshal(line, &one); err != nil {
			return nil, fmt.Errorf("parse line: %w", err)
		}
		out = append(out, one)
	}
	return out, nil
}
