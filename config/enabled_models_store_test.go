package config

import (
	"path/filepath"
	"testing"
)

// The allowlist is tri-state and a restart must not flatten it. In particular
// "disable all" (an empty, present allowlist) must not come back as "allow
// all" — a single TEXT DEFAULT '' column could not tell those apart.
func TestEnabledModelsSurviveReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kiro.db")
	st, err := newSQLStore("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Password: "x",
		Port:     8080,
		Accounts: []Account{
			{ID: "none", Enabled: true, EnabledModels: []string{}},
			{ID: "some", Enabled: true, EnabledModels: []string{"gpt-6-astra", "gpt-6-sol"}},
			{ID: "all", Enabled: true},
		},
	}
	if err := st.Save(cfg); err != nil {
		t.Fatal(err)
	}
	st.Close()

	st2, err := newSQLStore("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	got, err := st2.Load()
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]Account{}
	for _, a := range got.Accounts {
		byID[a.ID] = a
	}
	if a := byID["all"]; a.EnabledModels != nil {
		t.Errorf("no allowlist must reload as nil, got %#v", a.EnabledModels)
	}
	if a := byID["none"]; a.EnabledModels == nil || len(a.EnabledModels) != 0 {
		t.Errorf(`"disable all" must reload as empty-but-present, got %#v`, a.EnabledModels)
	} else if a.ServesModel("gpt-6-astra") {
		t.Error("an empty allowlist must still route nothing after a reload")
	}
	if a := byID["some"]; len(a.EnabledModels) != 2 || !a.ServesModel("gpt-6-sol") || a.ServesModel("glm-5.3") {
		t.Errorf("subset allowlist did not survive: %#v", a.EnabledModels)
	}
}
