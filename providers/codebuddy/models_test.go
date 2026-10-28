package codebuddy

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"hekato-go/config"
	"hekato-go/providers"
)

type catalogRoundTrip func(*http.Request) (*http.Response, error)

func (f catalogRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func catalogResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestModelsForAccountFetchesRegionalCatalog(t *testing.T) {
	for _, tt := range []struct{ name, region, host, domain, token string }{
		{"china", "cn", "copilot.tencent.com", "www.codebuddy.cn", "eyJheader.payload.signature"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			restore := providers.SwapClientsForTest(nil, &http.Client{Transport: catalogRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != tt.host || r.URL.Path != "/v3/config" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer "+tt.token || r.Header.Get("X-Domain") != tt.domain {
					t.Fatalf("unexpected auth/domain headers")
				}
				if isJWT(tt.token) && r.Header.Get("X-API-Key") != "" {
					t.Fatal("JWT sent as API key")
				}
				if !isJWT(tt.token) && r.Header.Get("X-API-Key") != tt.token {
					t.Fatal("missing API key header")
				}
				if r.Header.Get("X-User-Id") != "user-1" {
					t.Fatal("missing user identity")
				}
				if _, ok := r.Context().Deadline(); !ok {
					t.Fatal("catalog request must have a deadline")
				}
				return catalogResponse(200, `{"code":0,"data":{"models":[
     {"id":" new-model ","name":"New Model","descriptionEn":"Live model","supportsImages":true,"maxInputTokens":272000,"maxOutputTokens":32000},
     {"id":"text-model","descriptionZh":"模型","supportsImages":true,"disabledMultimodal":true},
     {"id":"NEW-MODEL","name":"Duplicate"},{"id":"  "}
    ]}}`), nil
			})})
			defer restore()
			got := ModelsForAccount(&config.Account{ID: tt.name, ProviderKind: "codebuddy", Region: tt.region, AccessToken: tt.token, UserId: "user-1"})
			if calls != 1 {
				t.Fatalf("made %d requests, want 1", calls)
			}
			if len(got) != 2 {
				t.Fatalf("got %d models, want only 2 live models", len(got))
			}
			if got[0].ModelId != "new-model" || got[0].ModelName != "New Model" || got[0].Description != "Live model" || !reflect.DeepEqual(got[0].InputTypes, []string{"text", "image"}) {
				t.Fatalf("incorrect model metadata: %+v", got[0])
			}
			if got[0].TokenLimits == nil || got[0].TokenLimits.MaxInputTokens != 272000 || got[0].TokenLimits.MaxOutputTokens != 32000 {
				t.Fatalf("incorrect token limits: %+v", got[0])
			}
			if got[1].ModelName != "text-model" || got[1].Description != "模型" || !reflect.DeepEqual(got[1].InputTypes, []string{"text"}) {
				t.Fatalf("incorrect text-only metadata: %+v", got[1])
			}
		})
	}
}

func TestModelsForAccountFallsBackOnCatalogFailure(t *testing.T) {
	for _, region := range []string{"global", "cn"} {
		for _, tt := range []struct {
			name, body string
			status     int
			err        error
		}{
			{name: "network", err: errors.New("unreachable")},
			{name: "unauthorized", status: 401, body: `{"message":"unauthorized"}`},
			{name: "upstream", status: 503, body: `unavailable`},
			{name: "application error", status: 200, body: `{"code":14001,"data":{"models":[{"id":"not-valid"}]}}`},
			{name: "malformed", status: 200, body: `{"data":`},
			{name: "empty", status: 200, body: `{"code":0,"data":{"models":[]}}`},
			{name: "invalid IDs", status: 200, body: `{"data":{"models":[{"id":"  "}]}}`},
		} {
			t.Run(region+"/"+tt.name, func(t *testing.T) {
				restore := providers.SwapClientsForTest(nil, &http.Client{Transport: catalogRoundTrip(func(*http.Request) (*http.Response, error) {
					if tt.err != nil {
						return nil, tt.err
					}
					return catalogResponse(tt.status, tt.body), nil
				})})
				defer restore()
				account := &config.Account{Region: region}
				want := ModelsForAccount(account) // no token: existing regional offline catalog
				account.AccessToken = "sk-test"
				got := ModelsForAccount(account)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("failed refresh did not return regional fallback")
				}
			})
		}
	}
}

func TestModelsForAccountRefreshReplacesCatalog(t *testing.T) {
	id := "first-live-model"
	restore := providers.SwapClientsForTest(nil, &http.Client{Transport: catalogRoundTrip(func(*http.Request) (*http.Response, error) {
		return catalogResponse(200, `{"data":{"models":[{"id":"`+id+`"}]}}`), nil
	})})
	defer restore()
	account := &config.Account{ID: "refresh", Region: "cn", AccessToken: "sk-test"}
	first := ModelsForAccount(account)
	id = "second-live-model"
	second := ModelsForAccount(account)
	if len(first) != 1 || first[0].ModelId != "first-live-model" || len(second) != 1 || second[0].ModelId != "second-live-model" {
		t.Fatal("refresh must replace the live catalog, not merge static or stale models")
	}
}

func TestModelsForAccountMissingCredentialsStaysOffline(t *testing.T) {
	restore := providers.SwapClientsForTest(nil, &http.Client{Transport: catalogRoundTrip(func(*http.Request) (*http.Response, error) {
		t.Fatal("missing credentials must not trigger a network request")
		return nil, errors.New("unexpected request")
	})})
	defer restore()
	for _, account := range []*config.Account{nil, {}, {Region: "cn"}} {
		if got := ModelsForAccount(account); len(got) == 0 {
			t.Fatal("missing credentials must still return the offline catalog")
		}
		if _, err := FetchModels(account); err == nil {
			t.Fatal("raw discovery should report missing credentials")
		}
	}
}

type catalogReadError struct{}

func (catalogReadError) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestFetchModelsRejectsUnreadableOrOversizedConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name string
		body io.Reader
	}{
		{"read error", catalogReadError{}},
		{"size limit", io.MultiReader(strings.NewReader(`{"data":{"models":[{"id":"valid"}]},"padding":"`), strings.NewReader(strings.Repeat("x", 16<<20)), strings.NewReader(`"}`))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			restore := providers.SwapClientsForTest(nil, &http.Client{Transport: catalogRoundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(tt.body)}, nil
			})})
			defer restore()
			if models, err := FetchModels(&config.Account{AccessToken: "sk-test"}); err == nil || len(models) != 0 {
				t.Fatalf("invalid configuration accepted: %d models, error %v", len(models), err)
			}
		})
	}
}

func TestFetchModelsUsesCLIAgentPicker(t *testing.T) {
	restore := providers.SwapClientsForTest(nil, &http.Client{Transport: catalogRoundTrip(func(*http.Request) (*http.Response, error) {
		return catalogResponse(200, `{"code":0,"data":{
			"models":[{"id":"nes-1.1"},{"id":"glm-5.3"},{"id":"seedance-2.5"},{"id":"GPT-5.5","name":"GPT-5.5"}],
			"agents":[{"name":"Explore","models":["lite"]},{"name":"cli","models":["gpt-5.5","missing","glm-5.3","glm-5.3"]}]
		}}`), nil
	})})
	defer restore()
	got, err := FetchModels(&config.Account{AccessToken: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range got {
		ids = append(ids, m.ModelId)
	}
	if !reflect.DeepEqual(ids, []string{"GPT-5.5", "glm-5.3"}) {
		t.Fatalf("got %v, want CLI picker order without internal models", ids)
	}
}

func TestModelsForAccountGlobalUsesCLIPicker(t *testing.T) {
	restore := providers.SwapClientsForTest(nil, &http.Client{Transport: catalogRoundTrip(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("global catalog must not be fetched: %s", r.URL)
		return nil, nil
	})})
	defer restore()
	got := ModelsForAccount(&config.Account{Region: "global", AccessToken: "sk-global"})
	if len(got) != len(codeBuddyGlobalModels) || got[0].ModelId != "default-model" {
		t.Fatalf("got %d models starting %q, want the CLI picker", len(got), got[0].ModelId)
	}
}

func TestCatalogCacheEvictsOldestAtBound(t *testing.T) {
	InvalidateModelCatalog()
	t.Cleanup(InvalidateModelCatalog)
	for i := 0; i < maxCatalogEntries+10; i++ {
		storeModelsCache(&config.Account{ID: fmt.Sprintf("acc-%03d", i)}, []providers.ModelInfo{{ModelId: "m"}})
	}
	catalogMu.Lock()
	size := len(catalogCache)
	// the 10 oldest must be gone, the newest must remain
	_, oldestGone := catalogCache["acc-000"]
	_, newestPresent := catalogCache[fmt.Sprintf("acc-%03d", maxCatalogEntries+9)]
	catalogMu.Unlock()
	if size != maxCatalogEntries {
		t.Fatalf("cache size %d, want %d", size, maxCatalogEntries)
	}
	if oldestGone {
		t.Fatal("oldest entry survived eviction")
	}
	if !newestPresent {
		t.Fatal("newest entry was evicted")
	}
}

func TestCatalogCacheConcurrentStores(t *testing.T) {
	InvalidateModelCatalog()
	t.Cleanup(InvalidateModelCatalog)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				storeModelsCache(&config.Account{ID: fmt.Sprintf("w%d-%d", w, i%20)}, []providers.ModelInfo{{ModelId: "m"}})
				cachedModelsFor(&config.Account{ID: fmt.Sprintf("w%d-%d", w, i%20)})
			}
		}(w)
	}
	wg.Wait()
}
