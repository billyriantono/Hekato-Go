package codex

import (
	"bytes"
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

// init registers the admin import endpoint so the dashboard can POST a 9router
// / etteum-pool token JSON to /auth/codex/import and have the account land in
// the pool. Mirrors providers/grok/routes.init.
func init() {
	providers.RegisterAdminRoutes(map[string]providers.RouteHandler{
		"POST /auth/codex/import": importCodex,
	})
}

// codexImportAccount is the per-account shape produced by 9router's
// auth.openai.com importer (and by etteum-pool's auth.codex module). We accept
// the raw OAuth token JSON the user gets after auth.openai.com redirects back
// to the CLI — no extra fields are required.
type codexImportAccount struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    string `json:"expires_at"`
	// Optional profile fields; some importers populate, others don't.
	Email     string `json:"email"`
	ID        string `json:"id"`
	IDToken   string `json:"id_token"`
	ClientID  string `json:"client_id"`
	Scope     string `json:"scope"`
	AccountID string `json:"account_id"`
}

// UnmarshalJSON accepts both the canonical snake_case shape (auth.openai.com
// OAuth responses, etteum-pool exports) and the camelCase shape produced by
// 9router's account dump. Snake_case wins on conflict; camelCase only fills
// fields the snake_case keys left empty. This keeps backward-compat with the
// admin import endpoint while letting operators paste a 9router export as-is.
func (c *codexImportAccount) UnmarshalJSON(data []byte) error {
	type alias codexImportAccount
	var primary alias
	if err := json.Unmarshal(data, &primary); err != nil {
		return err
	}
	*c = codexImportAccount(primary)

	var fallback map[string]json.RawMessage
	if err := json.Unmarshal(data, &fallback); err != nil {
		// Body was valid against the alias but not as a generic object — very
		// unusual; primary parse already succeeded so don't surface this.
		return nil
	}
	if c.AccessToken == "" {
		if v, ok := fallback["accessToken"]; ok {
			_ = json.Unmarshal(v, &c.AccessToken)
		}
	}
	if c.RefreshToken == "" {
		if v, ok := fallback["refreshToken"]; ok {
			_ = json.Unmarshal(v, &c.RefreshToken)
		}
	}
	if c.ExpiresIn == 0 {
		if v, ok := fallback["expiresIn"]; ok {
			_ = json.Unmarshal(v, &c.ExpiresIn)
		}
	}
	if c.ExpiresAt == "" {
		if v, ok := fallback["expiresAt"]; ok {
			_ = json.Unmarshal(v, &c.ExpiresAt)
		}
	}
	if c.ID == "" {
		if v, ok := fallback["id"]; ok {
			_ = json.Unmarshal(v, &c.ID)
		}
		if c.ID == "" {
			if v, ok := fallback["userId"]; ok {
				_ = json.Unmarshal(v, &c.ID)
			}
		}
	}
	if c.AccountID == "" {
		if v, ok := fallback["accountId"]; ok {
			_ = json.Unmarshal(v, &c.AccountID)
		}
		if c.AccountID == "" {
			if v, ok := fallback["account_id"]; ok {
				_ = json.Unmarshal(v, &c.AccountID)
			}
		}
	}
	return nil
}

// importCodex handles POST /auth/codex/import. Accepts:
//   - a single codexImportAccount object
//   - a JSON array of codexImportAccount
//   - newline-delimited JSON (one account per line)
//
// Each entry becomes a config.Account with AuthMethod "codex". Refresh-token
// imports are deliberately trust-on-import: we don't round-trip
// auth.RefreshToken because the user may have a fresh access token that hasn't
// yet been used (first call returns 200 with no rate-limit risk).
func importCodex(host providers.Host, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "read body failed: " + err.Error()})
		return
	}
	entries, err := parseCodexImportEntries(raw)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if len(entries) == 0 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "no codex account entries found"})
		return
	}

	imported := make([]map[string]interface{}, 0, len(entries))
	errs := make([]string, 0)
	for i, e := range entries {
		access := strings.TrimSpace(e.AccessToken)
		refresh := strings.TrimSpace(e.RefreshToken)
		if access == "" && refresh == "" {
			errs = append(errs, fmt.Sprintf("entry %d: missing access_token and refresh_token", i))
			continue
		}
		if access == "" {
			// Pure refresh-token import: we can't get an access token without
			// hitting auth.openai.com, but the operator may want to keep the
			// account around so the next RefreshToken round-trip uses it.
			// Store the refresh token only and let refresh on first use mint
			// the access token.
			access = ""
		}

		email := strings.TrimSpace(e.Email)
		userID := strings.TrimSpace(e.ID)
		if userID == "" {
			userID = strings.TrimSpace(e.AccountID)
		}

		// Best-effort userinfo lookup if we have a fresh access token — fills in
		// email / user id when the importer didn't. Best-effort, never fatal.
		if access != "" && (email == "" || userID == "") {
			if gotEmail, gotID, uerr := auth.FetchCodexUserInfo(access); uerr == nil {
				if email == "" {
					email = gotEmail
				}
				if userID == "" {
					userID = gotID
				}
			} else {
				logger.Warnf("[codex] userinfo lookup failed for entry %d: %v", i, uerr)
			}
		}

		var expiresAt int64
		if e.ExpiresAt != "" {
			if t, perr := time.Parse(time.RFC3339Nano, e.ExpiresAt); perr == nil {
				expiresAt = t.Unix()
			}
		}
		if expiresAt == 0 && e.ExpiresIn > 0 {
			expiresAt = time.Now().Unix() + int64(e.ExpiresIn)
		}
		if expiresAt == 0 && access != "" {
			expiresAt = auth.ExpFromAccessTokenJWT(access)
		}
		if expiresAt == 0 {
			// Last-resort default — better to over-refresh than to serve a
			// token we have no expiry for.
			expiresAt = time.Now().Add(45 * time.Minute).Unix()
		}

		account := config.Account{
			ID:           auth.GenerateAccountID(),
			Email:        email,
			Nickname:     email,
			AccessToken:  access,
			RefreshToken: refresh,
			AuthMethod:   "codex",
			Provider:     "codex",
			UserId:       userID,
			ExpiresAt:    expiresAt,
			Enabled:      true,
			MachineId:    config.GenerateMachineId(),
		}
		if account.Email == "" {
			account.Email = "codex account"
		}

		if err := config.AddAccount(account); err != nil {
			errs = append(errs, fmt.Sprintf("entry %d: %v", i, err))
			continue
		}
		if err := host.RefreshAccountModels(&account); err != nil {
			logger.Warnf("[codex] model cache refresh failed for %s: %v", account.Email, err)
		}
		imported = append(imported, map[string]interface{}{
			"id":    account.ID,
			"email": account.Email,
		})
	}

	host.ReloadPool()

	if len(imported) == 0 && len(errs) > 0 {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   strings.Join(errs, "; "),
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"imported": len(imported),
		"accounts": imported,
		"errors":   errs,
	})
}

// parseCodexImportEntries accepts a single JSON object, a JSON array, or
// newline-delimited JSON objects (one per line, blank lines skipped). Mirrors
// parseGrokImportEntries exactly so the dashboard can reuse the same upload UX.
func parseCodexImportEntries(raw []byte) ([]codexImportAccount, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty body")
	}
	if trimmed[0] == '[' {
		var arr []codexImportAccount
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, fmt.Errorf("parse array: %w", err)
		}
		return arr, nil
	}
	if trimmed[0] == '{' && singleTopLevelObject(trimmed) {
		// Verify the body is exactly ONE top-level object before taking the
		// direct branch. Anything past the matching `}` (e.g. a second object
		// on its own line for NDJSON) falls through to the line splitter. The
		// walker respects JSON strings so braces inside JWT tokens don't fool it.
		var one codexImportAccount
		if err := json.Unmarshal(trimmed, &one); err != nil {
			return nil, fmt.Errorf("parse object: %w", err)
		}
		return []codexImportAccount{one}, nil
	}
	var out []codexImportAccount
	for _, line := range bytes.Split(trimmed, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var one codexImportAccount
		if err := json.Unmarshal(line, &one); err != nil {
			return nil, fmt.Errorf("parse line: %w", err)
		}
		out = append(out, one)
	}
	return out, nil
}

// singleTopLevelObject reports whether body contains exactly one JSON object
// from offset 0 to its matching close, with only JSON whitespace after it.
// Handles nested objects/arrays and skips strings (so braces inside JWT-style
// tokens are not mistaken for structural braces).
func singleTopLevelObject(body []byte) bool {
	depth := 0
	inString := false
	escape := false
	started := false
	for i := range len(body) {
		c := body[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			switch c {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			depth++
			started = true
		case '}', ']':
			depth--
			if started && depth == 0 {
				// matched the outermost close; everything after must be whitespace
				for j := i + 1; j < len(body); j++ {
					if body[j] != ' ' && body[j] != '\t' && body[j] != '\n' && body[j] != '\r' {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}
