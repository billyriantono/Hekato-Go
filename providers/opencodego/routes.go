package opencodego

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
)

func init() {
	providers.RegisterAdminRoutes(map[string]providers.RouteHandler{
		"opencodego/import": importOpenCodeGo,
	})
}

// importAccount is the per-entry body. `access_token` is required; email/nickname
// are optional and only used to label the account in the admin panel.
type importAccount struct {
	AccessToken string `json:"access_token"`
	Email       string `json:"email"`
	Nickname    string `json:"nickname"`
	ID          string `json:"id"`
}

// importOpenCodeGo handles POST /auth/opencodego/import. Accepts:
//   - a single importAccount JSON object
//   - a JSON array of importAccount
//   - newline-delimited bare API keys (one per line)
func importOpenCodeGo(host providers.Host, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "read body failed: "+err.Error())
		return
	}

	entries, err := parseEntries(raw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(entries) == 0 {
		writeJSONError(w, http.StatusBadRequest, "no opencodego entries found")
		return
	}

	imported := make([]map[string]interface{}, 0, len(entries))
	importErrors := make([]string, 0)
	for i, e := range entries {
		access := strings.TrimSpace(e.AccessToken)
		if access == "" {
			importErrors = append(importErrors, fmt.Sprintf("entry %d: missing access_token", i))
			continue
		}

		email := strings.TrimSpace(e.Email)
		if email == "" {
			email = strings.TrimSpace(e.Nickname)
		}
		if email == "" {
			email = "opencode-go account"
		}
		nickname := strings.TrimSpace(e.Nickname)
		if nickname == "" {
			nickname = email
		}

		account := config.Account{
			ID:           auth.GenerateAccountID(),
			Email:        email,
			Nickname:     nickname,
			AccessToken:  access,
			AuthMethod:   "opencode_go",
			Provider:     "opencode_go",
			ProviderKind: "opencode_go",
			Enabled:      true,
			MachineId:    config.GenerateMachineId(),
		}

		if err := config.AddAccount(account); err != nil {
			importErrors = append(importErrors, fmt.Sprintf("entry %d: %v", i, err))
			continue
		}
		if err := host.RefreshAccountModels(&account); err != nil {
			logger.Warnf("[opencodego] model refresh failed for %s: %v", account.Email, err)
		}
		imported = append(imported, map[string]interface{}{
			"id":    account.ID,
			"email": account.Email,
		})
	}

	host.ReloadPool()

	if len(imported) == 0 && len(importErrors) > 0 {
		writeJSONError(w, http.StatusInternalServerError, strings.Join(importErrors, "; "))
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"imported": len(imported),
		"accounts": imported,
		"errors":   importErrors,
	})
}

func parseEntries(raw []byte) ([]importAccount, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, nil
	}
	if trimmed[0] == '{' {
		var single importAccount
		if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
			return nil, fmt.Errorf("invalid JSON object: %w", err)
		}
		return []importAccount{single}, nil
	}
	if trimmed[0] == '[' {
		var arr []importAccount
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, fmt.Errorf("invalid JSON array: %w", err)
		}
		return arr, nil
	}
	var out []importAccount
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, importAccount{AccessToken: line})
	}
	return out, nil
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "success": "false"})
}