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

// The probe model drives both the Test button and warmup. A disabled model
// must never be probed: it proves nothing about the account and reports a
// failure the operator deliberately arranged.
func TestProbeModelRespectsAllowlist(t *testing.T) {
	if err := config.Init(t.TempDir() + "/config.json"); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateTestModel("claude-opus-5"); err != nil {
		t.Fatal(err)
	}

	// A stale probe model, left behind when the operator narrowed the allowlist.
	acc := &config.Account{
		ID: "acc", Enabled: true,
		ProbeModel:    "glm-5.3-flash",
		EnabledModels: []string{"gpt-6-astra"},
	}
	if got := probeModelFor(acc); got != "gpt-6-astra" {
		t.Errorf("probe should fall back to an allowed model, got %q", got)
	}

	// The global default is equally subject to the allowlist.
	acc.ProbeModel = ""
	if got := probeModelFor(acc); got == "claude-opus-5" {
		t.Error("the global test model must not override the allowlist")
	}

	// An allowed probe model is honoured untouched.
	acc.ProbeModel = "gpt-6-astra"
	if got := probeModelFor(acc); got != "gpt-6-astra" {
		t.Errorf("an allowed probe model must be used, got %q", got)
	}

	// Without an allowlist nothing changes.
	open := &config.Account{ID: "open", Enabled: true, ProbeModel: "glm-5.3-flash"}
	if got := probeModelFor(open); got != "glm-5.3-flash" {
		t.Errorf("accounts without an allowlist keep their probe model, got %q", got)
	}
}

// The allowlist is tri-state and the API must keep the three apart:
// absent = unchanged, null = no allowlist, [] = allow nothing.
func TestAllowlistTriState(t *testing.T) {
	if err := config.Init(t.TempDir() + "/config.json"); err != nil {
		t.Fatal(err)
	}
	if err := config.AddAccount(config.Account{ID: "acc", Enabled: true, AccessToken: "t"}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{pool: accountpool.GetPool()}
	update := func(body string) *config.Account {
		rec := httptest.NewRecorder()
		h.apiUpdateAccount(rec, httptest.NewRequest("PUT", "/admin/api/accounts/acc", strings.NewReader(body)), "acc")
		if rec.Code != 200 {
			t.Fatalf("update %s failed: %d %s", body, rec.Code, rec.Body.String())
		}
		for _, a := range config.GetAccounts() {
			if a.ID == "acc" {
				cp := a
				return &cp
			}
		}
		t.Fatal("account vanished")
		return nil
	}

	if got := update(`{"enabledModels":["gpt-6-astra"]}`); !got.ServesModel("gpt-6-astra") || got.ServesModel("glm-5.3") {
		t.Errorf("subset allowlist not applied: %v", got.EnabledModels)
	}
	// Disable all: an allowlist that permits nothing, not a cleared one.
	got := update(`{"enabledModels":[]}`)
	if got.EnabledModels == nil {
		t.Fatal("an empty allowlist must persist as empty, not nil")
	}
	if got.ServesModel("gpt-6-astra") || got.ServesModel("anything") {
		t.Error("an empty allowlist must route nothing")
	}
	// Unrelated updates leave it alone.
	if got := update(`{"nickname":"x"}`); got.EnabledModels == nil {
		t.Error("an update that omits the field must not clear it")
	}
	// null restores the default.
	if got := update(`{"enabledModels":null}`); got.EnabledModels != nil || !got.ServesModel("anything") {
		t.Errorf("null must clear the allowlist, got %v", got.EnabledModels)
	}
}
