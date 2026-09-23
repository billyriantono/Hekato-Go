package clinepass

import (
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/providers"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Cline account API (api.cline.bot):
//
//	GET /api/v1/users/me                 → {data:{id,email,displayName,organizations[]}}
//	GET /api/v1/users/{id}/balance       → {data:{userId,balance}}
//	GET /api/v1/users/{id}/usages        → {data:{items:[{createdAt,costUsd,creditsUsed,...}],nextToken,total}}
//
// Monetary fields are integers in 1e-8 USD (a 249k-token kimi-k3 call is
// billed 17,312,880 → $0.17). ClinePass is a flat subscription, so there is
// no hard quota to expose: UsageCurrent is month-to-date spend in USD and
// the credit balance is surfaced as the trial/extra figure.
const clineMoneyUnit = 1e8

func clineGET(account *config.Account, path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, clinepassBaseURL+path, nil)
	if err != nil {
		return err
	}
	setClinepassHeaders(req, account)
	req.Header.Set("Accept", "application/json")
	resp, err := providers.GetRestClientForAccount(account).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return providers.Errorf(resp.StatusCode, "clinepass %s: HTTP %d %s", path, resp.StatusCode, msg)
	}
	return json.Unmarshal(body, out)
}

// FetchUsage reports identity, credit balance and month-to-date spend.
func FetchUsage(account *config.Account) (*config.AccountInfo, error) {
	if account == nil || strings.TrimSpace(account.AccessToken) == "" {
		return nil, fmt.Errorf("clinepass: account has no access token")
	}
	var me struct {
		Data struct {
			ID            string `json:"id"`
			Email         string `json:"email"`
			DisplayName   string `json:"displayName"`
			Organizations []struct {
				ID string `json:"id"`
			} `json:"organizations"`
		} `json:"data"`
	}
	if err := clineGET(account, "/api/v1/users/me", &me); err != nil {
		return nil, err
	}
	if me.Data.ID == "" {
		return nil, fmt.Errorf("clinepass: /users/me returned no user id")
	}

	info := &config.AccountInfo{
		LastRefresh:       time.Now().Unix(),
		Email:             me.Data.Email,
		UserId:            me.Data.ID,
		SubscriptionType:  "CLINE_PASS",
		SubscriptionTitle: "Cline Pass",
	}

	var bal struct {
		Data struct {
			Balance float64 `json:"balance"`
		} `json:"data"`
	}
	if err := clineGET(account, "/api/v1/users/"+url.PathEscape(me.Data.ID)+"/balance", &bal); err == nil {
		// Credit balance (USD); negative means the pass absorbed overage.
		info.TrialUsageCurrent = bal.Data.Balance / clineMoneyUnit
		info.TrialStatus = "BALANCE"
	}

	// Month-to-date spend: page through usages until we leave the month.
	monthStart := time.Now().UTC().Truncate(24 * time.Hour)
	monthStart = time.Date(monthStart.Year(), monthStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	var spend float64
	next := ""
	for page := 0; page < 20; page++ {
		path := "/api/v1/users/" + url.PathEscape(me.Data.ID) + "/usages"
		if next != "" {
			path += "?nextToken=" + url.QueryEscape(next)
		}
		var res struct {
			Data struct {
				Items []struct {
					CreatedAt string  `json:"createdAt"`
					CostUsd   float64 `json:"costUsd"`
				} `json:"items"`
				NextToken string `json:"nextToken"`
			} `json:"data"`
		}
		if err := clineGET(account, path, &res); err != nil {
			break
		}
		older := false
		for _, it := range res.Data.Items {
			t, err := time.Parse(time.RFC3339Nano, it.CreatedAt)
			if err == nil && t.Before(monthStart) {
				older = true
				continue
			}
			spend += it.CostUsd / clineMoneyUnit
		}
		if older || res.Data.NextToken == "" || len(res.Data.Items) == 0 {
			break
		}
		next = res.Data.NextToken
	}
	info.UsageCurrent = spend
	info.NextResetDate = monthStart.AddDate(0, 1, 0).Format("2006-01-02")
	return info, nil
}
