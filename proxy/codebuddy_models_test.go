package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hekato-go/config"
	"hekato-go/pool"
	"hekato-go/providers"
)

type codeBuddyModelsTransport func(*http.Request) (*http.Response, error)

func (f codeBuddyModelsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func setupCodeBuddyModelHandler(t *testing.T, accounts ...config.Account) *Handler {
	t.Helper()
	if err := config.Init(t.TempDir() + "/config.json"); err != nil {
		t.Fatal(err)
	}
	p := pool.GetPool()
	for _, a := range accounts {
		if err := config.AddAccount(a); err != nil {
			t.Fatal(err)
		}
		p.SetModelList(a.ID, nil)
	}
	p.Reload()
	t.Cleanup(func() {
		for _, a := range accounts {
			p.SetModelList(a.ID, nil)
		}
	})
	return &Handler{pool: p}
}

func codeBuddyModelIDs(models []ModelInfo) map[string]bool {
	out := map[string]bool{}
	for _, m := range models {
		out[m.ModelId] = true
	}
	return out
}
func codeBuddyPoolIDs(h *Handler, id string) map[string]bool {
	out := map[string]bool{}
	for _, m := range h.pool.GetModelList(id) {
		out[m] = true
	}
	return out
}

func TestCodeBuddyAccountRefreshReplacesRetiredAggregateModels(t *testing.T) {
	for _, endpoint := range []string{"fetch", "admin-get"} {
		t.Run(endpoint, func(t *testing.T) {
			a := config.Account{ID: "cb-refresh", ProviderKind: "codebuddy", AccessToken: "sk-test", Enabled: true,
				EnabledModels: []string{"old", "new", "shared", "custom", "extra"}, ExtraModels: []string{"extra"}}
			b := config.Account{ID: "cb-other", ProviderKind: "codebuddy", AccessToken: "sk-other", Enabled: true}
			h := setupCodeBuddyModelHandler(t, a, b)
			if err := config.UpdateCustomModelIDs("codebuddy", []string{"custom"}); err != nil {
				t.Fatal(err)
			}
			h.pool.SetModelList(b.ID, []string{"shared", "restored-text"})
			h.cachedModels = []ModelInfo{{ModelId: "shared", ModelName: "Restored shared", InputTypes: []string{"text", "image"}}}
			phase := 0
			restore := providers.SwapClientsForTest(nil, &http.Client{Transport: codeBuddyModelsTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/v3/config" || r.Method != http.MethodGet {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				ids := []string{"old", "shared", "blocked"}
				if phase == 1 {
					ids = []string{"new", "blocked"}
				}
				entries := make([]map[string]string, 0, len(ids))
				for _, id := range ids {
					entries = append(entries, map[string]string{"id": id})
				}
				raw, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{"models": entries}})
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
			})})
			defer restore()
			refresh := func() {
				t.Helper()
				if endpoint == "fetch" {
					if err := h.fetchAndCacheAccountModels(&a); err != nil {
						t.Fatal(err)
					}
				} else {
					rec := httptest.NewRecorder()
					h.apiGetAccountModels(rec, httptest.NewRequest("GET", "/admin/api/accounts/"+a.ID+"/models", nil), a.ID)
					if rec.Code != 200 {
						t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
					}
				}
			}
			refresh()
			phase = 1
			refresh()
			ids := codeBuddyPoolIDs(h, a.ID)
			if ids["old"] || ids["shared"] || !ids["new"] || !ids["blocked"] || !ids["custom"] || !ids["extra"] {
				t.Errorf("pool catalog not replaced or extras lost: %v", ids)
			}
			advertised := codeBuddyModelIDs(h.cachedModels)
			if advertised["old"] || advertised["blocked"] || !advertised["new"] || !advertised["shared"] || !advertised["custom"] || !advertised["extra"] {
				t.Errorf("aggregate contains retired/disallowed models or lost another account: %v", advertised)
			}
			if !advertised["restored-text"] || !h.modelHasVision("shared") {
				t.Errorf("restored account metadata or basic catalog entry lost: %+v", h.cachedModels)
			}
			saved, err := config.Blobs().LoadBlob("model-lists")
			if err != nil || !strings.Contains(saved, `"new"`) || strings.Contains(saved, `"old"`) {
				t.Errorf("refreshed catalogs not persisted: %s, %v", saved, err)
			}
		})
	}
}

func refreshCodeBuddyTestAccount(t *testing.T, h *Handler, account *config.Account, endpoint string) {
	t.Helper()
	if endpoint == "fetch" {
		if err := h.fetchAndCacheAccountModels(account); err != nil {
			t.Fatal(err)
		}
		return
	}
	rec := httptest.NewRecorder()
	h.apiGetAccountModels(rec, httptest.NewRequest("GET", "/admin/api/accounts/"+account.ID+"/models", nil), account.ID)
	if rec.Code != 200 {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCodeBuddyTargetedRefreshReplacesSameIDMetadata(t *testing.T) {
	for _, endpoint := range []string{"fetch", "admin-get"} {
		for _, initial := range []string{"live", "fallback"} {
			t.Run(endpoint+"/"+initial, func(t *testing.T) {
				a := config.Account{ID: "cb-metadata", ProviderKind: "codebuddy", AccessToken: "sk-meta", Enabled: true}
				h := setupCodeBuddyModelHandler(t, a)
				phase := 0
				restore := providers.SwapClientsForTest(nil, &http.Client{Transport: codeBuddyModelsTransport(func(*http.Request) (*http.Response, error) {
					status := http.StatusOK
					body := `{"code":0,"data":{"models":[{"id":"claude-opus-4.8","name":"Old name","supportsImages":true,"maxInputTokens":200000,"maxOutputTokens":8000}]}}`
					if phase == 0 && initial == "fallback" {
						status = http.StatusServiceUnavailable
					}
					if phase == 1 {
						body = `{"code":0,"data":{"models":[{"id":"claude-opus-4.8","name":"Current name","supportsImages":false,"maxInputTokens":64000,"maxOutputTokens":4000}]}}`
					}
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
				})})
				defer restore()
				refreshCodeBuddyTestAccount(t, h, &a, endpoint)
				phase = 1
				refreshCodeBuddyTestAccount(t, h, &a, endpoint)
				if len(h.cachedModels) != 1 {
					t.Fatalf("expected current live catalog, got %v", codeBuddyModelIDs(h.cachedModels))
				}
				model := h.cachedModels[0]
				if model.ModelName != "Current name" || model.TokenLimits == nil || model.TokenLimits.MaxInputTokens != 64000 || model.TokenLimits.MaxOutputTokens != 4000 || modelSupportsImage(model.InputTypes) {
					t.Fatalf("same-ID metadata remained stale: %+v limits=%+v", model, model.TokenLimits)
				}
			})
		}
	}
}

func TestCodeBuddyTargetedRefreshPreservesSharedAccountMetadata(t *testing.T) {
	for _, endpoint := range []string{"fetch", "admin-get"} {
		t.Run(endpoint, func(t *testing.T) {
			a := config.Account{ID: "cb-meta-a", ProviderKind: "codebuddy", AccessToken: "sk-a", Enabled: true}
			b := config.Account{ID: "cb-meta-b", ProviderKind: "codebuddy", AccessToken: "sk-b", Enabled: true}
			h := setupCodeBuddyModelHandler(t, a, b)
			images := map[string]bool{"Bearer sk-a": true, "Bearer sk-b": true}
			restore := providers.SwapClientsForTest(nil, &http.Client{Transport: codeBuddyModelsTransport(func(r *http.Request) (*http.Response, error) {
				body := fmt.Sprintf(`{"code":0,"data":{"models":[{"id":"shared","supportsImages":%t}]}}`, images[r.Header.Get("Authorization")])
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})})
			defer restore()
			// Populate both accounts' metadata through the periodic-refresh path.
			h.refreshModelsCache()
			images["Bearer sk-a"] = false
			refreshCodeBuddyTestAccount(t, h, &a, endpoint)
			if !h.modelHasVision("shared") {
				t.Fatal("refresh lost the other account's image support")
			}
			images["Bearer sk-b"] = false
			refreshCodeBuddyTestAccount(t, h, &b, endpoint)
			if h.modelHasVision("shared") {
				t.Fatal("image support survived after every account removed it")
			}
		})
	}
}

func TestCodeBuddyRefreshRegionalIsolationAndUnavailableFallback(t *testing.T) {
	global := config.Account{ID: "cb-global", ProviderKind: "codebuddy", Region: "global", AccessToken: "sk-global", Enabled: true}
	cn := config.Account{ID: "cb-cn", ProviderKind: "codebuddy", Region: "cn", AccessToken: "sk-cn", Enabled: true}
	h := setupCodeBuddyModelHandler(t, global, cn)
	unavailable := false
	restore := providers.SwapClientsForTest(nil, &http.Client{Transport: codeBuddyModelsTransport(func(r *http.Request) (*http.Response, error) {
		status, id := 200, "global-live"
		if r.URL.Host == "copilot.tencent.com" {
			id = "cn-live"
		} else if r.URL.Host != "www.codebuddy.ai" {
			t.Fatalf("wrong regional host: %s", r.URL.Host)
		}
		if unavailable {
			status = http.StatusUnauthorized
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"code":0,"data":{"models":[{"id":%q}]}}`, id)))}, nil
	})})
	defer restore()
	h.refreshModelsCache()
	if g, c := codeBuddyPoolIDs(h, global.ID), codeBuddyPoolIDs(h, cn.ID); !g["global-live"] || g["cn-live"] || !c["cn-live"] || c["global-live"] {
		t.Fatalf("regional catalogs leaked: global=%v cn=%v", g, c)
	}
	unavailable = true
	h.refreshModelsCache()
	if g, c := codeBuddyPoolIDs(h, global.ID), codeBuddyPoolIDs(h, cn.ID); !g["claude-opus-4.8"] || c["claude-opus-4.8"] || !c["glm-5.3"] || g["global-live"] || c["cn-live"] {
		t.Errorf("regional static fallback incorrect: global=%v cn=%v", g, c)
	}
	for _, a := range config.GetAccounts() {
		if !a.Enabled {
			t.Errorf("metadata failure disabled account %s", a.ID)
		}
	}
}
