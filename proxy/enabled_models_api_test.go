package proxy

import (
	"encoding/json"
	"hekato-go/config"
	accountpool "hekato-go/pool"
	"net/http/httptest"
	"strings"
	"testing"
)

// A model added by hand while an allowlist is active must become routable.
// Otherwise the operator adds it, sees it listed, and it is silently refused
// by an allowlist that predates it.
func TestManuallyAddedModelJoinsActiveAllowlist(t *testing.T) {
	if err := config.Init(t.TempDir() + "/config.json"); err != nil {
		t.Fatal(err)
	}
	if err := config.AddAccount(config.Account{
		ID: "acc", Email: "a@test", Enabled: true, AccessToken: "t",
		EnabledModels: []string{"gpt-6-astra"},
		ExtraModels:   []string{"gpt-6-astra"},
	}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{pool: accountpool.GetPool()}

	body := `{"extraModels":["gpt-6-astra","gpt-6-sol"]}`
	rec := httptest.NewRecorder()
	h.apiUpdateAccount(rec, httptest.NewRequest("PUT", "/admin/api/accounts/acc", strings.NewReader(body)), "acc")
	if rec.Code != 200 {
		t.Fatalf("update failed: %d %s", rec.Code, rec.Body.String())
	}

	var acc *config.Account
	for _, a := range config.GetAccounts() {
		if a.ID == "acc" {
			cp := a
			acc = &cp
		}
	}
	if acc == nil {
		t.Fatal("account vanished")
	}
	if !acc.ServesModel("gpt-6-sol") {
		t.Errorf("newly added model should be routable, allowlist is %v", acc.EnabledModels)
	}
	if !acc.ServesModel("gpt-6-astra") {
		t.Error("the original allowlist entry must survive")
	}
	if acc.ServesModel("claude-opus-5") {
		t.Error("the allowlist must still exclude everything else")
	}

	// Removing an entry from the allowlist stays possible afterwards.
	rec = httptest.NewRecorder()
	h.apiUpdateAccount(rec, httptest.NewRequest("PUT", "/admin/api/accounts/acc", strings.NewReader(`{"enabledModels":["gpt-6-sol"]}`)), "acc")
	if rec.Code != 200 {
		t.Fatalf("second update failed: %d", rec.Code)
	}
	for _, a := range config.GetAccounts() {
		if a.ID == "acc" && a.ServesModel("gpt-6-astra") {
			t.Error("an explicitly narrowed allowlist must be honoured")
		}
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
}
