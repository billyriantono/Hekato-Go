package proxy

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"hekato-go/config"
	accountpool "hekato-go/pool"
)

// A "<pool>/model" request is pinned to that pool, the key allowlist is
// checked against the pool-qualified name, and /v1/models lists prefixed IDs
// only for models more than one pool offers.
func TestPoolPrefixRouting(t *testing.T) {
	mustInitConfig(t)
	for _, a := range []config.Account{
		{ID: "pg", Email: "pg", AccessToken: "ck_x", RefreshToken: "ck_x", AuthMethod: "codebuddy", Provider: "CodeBuddy", Region: "global", Enabled: true},
		{ID: "pc", Email: "pc", AccessToken: "ck_x", RefreshToken: "ck_x", AuthMethod: "codebuddy-cn", Provider: "CodeBuddy CN", Region: "cn", Enabled: true},
	} {
		if err := config.AddAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("pg", []string{"deepseek-v4.1-flash"})
	p.SetModelList("pc", []string{"deepseek-v4.1-flash", "hy3"})
	t.Cleanup(func() { p.SetModelList("pg", nil); p.SetModelList("pc", nil) })
	if err := config.UpdatePoolPrefixes(map[string]string{"codebuddy-cn": "cn"}); err != nil {
		t.Fatal(err)
	}
	key, err := config.AddApiKey(config.ApiKeyEntry{Name: "cn-only", Key: "sk-cn", Enabled: true, AllowedModels: []string{"codebuddy-cn/*"}})
	if err != nil {
		t.Fatal(err)
	}

	// Routing: the custom prefix pins every pick to the CN account.
	w := &logMetaWriter{ResponseWriter: httptest.NewRecorder()}
	model, ok := pinModelPool(w, key.ID, "cn/deepseek-v4.1-flash")
	if !ok || model != "deepseek-v4.1-flash" {
		t.Fatalf("pinModelPool = %q, %v", model, ok)
	}
	for i := 0; i < 4; i++ {
		if acc := p.GetNextForModelExcluding(model, nil, requestFilter(w, capOpenAIChat)); acc == nil || acc.ID != "pc" {
			t.Fatalf("pinned pick %d = %v, want pc", i, acc)
		}
	}
	// Allowlist: the CN-only key may not use the bare ID or the global pool.
	for _, m := range []string{"deepseek-v4.1-flash", "codebuddy/deepseek-v4.1-flash"} {
		if _, ok := pinModelPool(&logMetaWriter{ResponseWriter: httptest.NewRecorder()}, key.ID, m); ok {
			t.Fatalf("cn-only key allowed %q", m)
		}
	}

	// An allowlist written with the custom prefix ("cn/*", what the picker
	// offers) must allow the same requests as the pool-key form.
	cnKey, err := config.AddApiKey(config.ApiKeyEntry{Name: "cn-prefix", Key: "sk-cnp", Enabled: true, AllowedModels: []string{"cn/*"}})
	if err != nil {
		t.Fatal(err)
	}
	if model, ok := pinModelPool(&logMetaWriter{ResponseWriter: httptest.NewRecorder()}, cnKey.ID, "cn/deepseek-v4.1-flash"); !ok || model != "deepseek-v4.1-flash" {
		t.Fatalf("custom-prefix allowlist refused its own pool: %q, %v", model, ok)
	}
	if _, ok := pinModelPool(&logMetaWriter{ResponseWriter: httptest.NewRecorder()}, cnKey.ID, "codebuddy/deepseek-v4.1-flash"); ok {
		t.Fatal("custom-prefix allowlist allowed another pool")
	}

	// Auto-route: a pool-qualified tier entry keeps only that pool's pair, and
	// a pool-scoped blacklist entry drops only that pool's pair.
	pairs := func(patterns, blacklist []string) map[string]bool {
		cfg := config.DefaultAutoRouteConfig()
		cfg.Blacklist = blacklist
		out := map[string]bool{}
		for _, c := range newAutoRouter().candidates(p, patterns, nil, cfg) {
			out[c.account.ID+"|"+c.model] = true
		}
		return out
	}
	if got := pairs([]string{"codebuddy/deepseek-v4.1-flash"}, nil); len(got) != 1 || !got["pg|deepseek-v4.1-flash"] {
		t.Fatalf("pool-qualified tier entry: %v", got)
	}
	if got := pairs([]string{"=cn/deepseek-v4.1-flash"}, nil); len(got) != 1 || !got["pc|deepseek-v4.1-flash"] {
		t.Fatalf("exact custom-prefix tier entry: %v", got)
	}
	if got := pairs([]string{"deepseek"}, []string{"cn:deepseek"}); len(got) != 1 || !got["pg|deepseek-v4.1-flash"] {
		t.Fatalf("pool blacklist: %v", got)
	}
	if got := pairs([]string{"deepseek"}, []string{"codebuddy:deepseek"}); len(got) != 0 {
		t.Fatalf("provider blacklist must still cover both pools: %v", got)
	}
	if !matchesTier("deepseek-v4.1-flash", []string{"codebuddy/deepseek-v4.1-flash"}, nil) {
		t.Fatal("without an account, a pool-qualified entry must match the bare model")
	}

	// Listing: deepseek is in both pools, hy3 only in CN.
	got := map[string]bool{}
	for _, pm := range (&Handler{pool: p}).poolModels() {
		got[pm.prefix+"/"+pm.model] = pm.multi
	}
	want := map[string]bool{"cn/deepseek-v4.1-flash": true, "codebuddy/deepseek-v4.1-flash": true, "cn/hy3": false}
	if len(got) != len(want) {
		t.Fatalf("poolModels = %v, want %v", got, want)
	}
	for id, multi := range want {
		if m, ok := got[id]; !ok || m != multi {
			t.Fatalf("poolModels = %v, want %v", got, want)
		}
	}
}

func TestWorkBuddyPoolIsolation(t *testing.T) {
	accounts := []config.Account{
		{ID: "wb-global", ProviderKind: "codebuddy", Region: "global", AccessToken: "fixture", Enabled: true},
		{ID: "wb-cn", ProviderKind: "codebuddy", Region: "cn", AccessToken: "fixture", Enabled: true},
		{ID: "wb-own", ProviderKind: "codebuddy", Provider: "WorkBuddy", Region: "workbuddy", AccessToken: "fixture", Enabled: true},
	}
	h := setupCodeBuddyModelHandler(t, accounts...)
	for _, a := range accounts {
		h.pool.SetModelList(a.ID, []string{"deepseek-v4.1-flash"})
	}
	rec := httptest.NewRecorder()
	h.apiGetAccounts(rec, httptest.NewRequest("GET", "/admin/api/accounts", nil))
	var rows []struct {
		ID   string `json:"id"`
		Pool string `json:"pool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	wantPools := map[string]string{"wb-global": "codebuddy", "wb-cn": "codebuddy-cn", "wb-own": "workbuddy"}
	if len(rows) != len(wantPools) {
		t.Fatalf("account API returned %d accounts, want %d", len(rows), len(wantPools))
	}
	for _, row := range rows {
		if row.Pool != wantPools[row.ID] {
			t.Fatalf("dashboard account %s pool = %q, want %q", row.ID, row.Pool, wantPools[row.ID])
		}
	}
	if err := config.UpdatePoolTestModels(map[string]string{"workbuddy": "deepseek-v4.1-flash"}); err != nil {
		t.Fatal(err)
	}
	if config.GetPoolTestModel("codebuddy") != "" || config.GetPoolTestModel("codebuddy-cn") != "" {
		t.Fatal("WorkBuddy test model leaked into another pool")
	}
	key, err := config.AddApiKey(config.ApiKeyEntry{Name: "wb-only", Key: "fixture-wb", Enabled: true, AllowedModels: []string{"workbuddy/*"}})
	if err != nil {
		t.Fatal(err)
	}
	for prefix, wantID := range map[string]string{"codebuddy": "wb-global", "codebuddy-cn": "wb-cn", "workbuddy": "wb-own"} {
		w := &logMetaWriter{ResponseWriter: httptest.NewRecorder()}
		model, ok := pinModelPool(w, "", prefix+"/deepseek-v4.1-flash")
		if !ok || model != "deepseek-v4.1-flash" {
			t.Fatalf("cannot pin %s: %q %v", prefix, model, ok)
		}
		for i := 0; i < 4; i++ {
			if a := h.pool.GetNextForModelExcluding(model, nil, requestFilter(w, capOpenAIChat)); a == nil || a.ID != wantID {
				t.Fatalf("%s picked %v, want %s", prefix, a, wantID)
			}
		}
		_, allowed := pinModelPool(&logMetaWriter{ResponseWriter: httptest.NewRecorder()}, key.ID, prefix+"/deepseek-v4.1-flash")
		if allowed != (prefix == "workbuddy") {
			t.Fatalf("WorkBuddy-only API key access to %s = %v", prefix, allowed)
		}
	}
	listed := map[string]bool{}
	for _, pm := range h.poolModels() {
		listed[pm.prefix+"/"+pm.model] = pm.multi
	}
	for _, prefix := range []string{"codebuddy", "codebuddy-cn", "workbuddy"} {
		if !listed[prefix+"/deepseek-v4.1-flash"] {
			t.Fatalf("missing separate multi-pool listing for %s: %v", prefix, listed)
		}
	}
	cfg := config.DefaultAutoRouteConfig()
	got := newAutoRouter().candidates(h.pool, []string{"workbuddy/deepseek-v4.1-flash"}, nil, cfg)
	if len(got) != 1 || got[0].account.ID != "wb-own" {
		t.Fatalf("WorkBuddy tier escaped its pool: %v", got)
	}
	cfg.Blacklist = []string{"workbuddy:deepseek"}
	got = newAutoRouter().candidates(h.pool, []string{"deepseek"}, nil, cfg)
	if len(got) != 2 {
		t.Fatalf("WorkBuddy blacklist affected other pools: %v", got)
	}
	for _, candidate := range got {
		if candidate.account.ID == "wb-own" {
			t.Fatal("WorkBuddy blacklist did not exclude its own pool")
		}
	}
	cfg.Blacklist = []string{"codebuddy:deepseek"}
	if got := newAutoRouter().candidates(h.pool, []string{"deepseek"}, nil, cfg); len(got) != 0 {
		t.Fatalf("shared-provider blacklist must still cover all variants: %v", got)
	}
}

func TestApplyModelToggles(t *testing.T) {
	cat := []string{"a", "b", "c"}
	for _, tc := range []struct {
		name               string
		cur, enable, dis   []string
		want               []string
		wantNil, wantDirty bool
	}{
		{"enable on no allowlist is a no-op", nil, []string{"a"}, nil, nil, true, false},
		{"disable on no allowlist starts from catalog", nil, nil, []string{"b"}, []string{"a", "c"}, false, true},
		{"disable without cached catalog is skipped", nil, nil, []string{"b"}, nil, true, false},
		{"re-enabling the last model collapses to nil", []string{"a", "c"}, []string{"B"}, nil, nil, true, true},
		{"disabling everything keeps an empty allowlist", []string{"a"}, nil, []string{"a"}, []string{}, false, true},
		{"unchanged list is not dirty", []string{"C", "a"}, []string{"a"}, nil, nil, true, false},
	} {
		catalog := cat
		if tc.name == "disable without cached catalog is skipped" {
			catalog = nil
		}
		got, dirty := applyModelToggles(tc.cur, catalog, tc.enable, tc.dis)
		if dirty != tc.wantDirty || !dirty {
			if dirty != tc.wantDirty {
				t.Errorf("%s: dirty = %v", tc.name, dirty)
			}
			continue
		}
		if (got == nil) != tc.wantNil || !tc.wantNil && strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: got %#v", tc.name, got)
		}
	}
}
