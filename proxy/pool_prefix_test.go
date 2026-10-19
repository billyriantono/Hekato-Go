package proxy

import (
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
