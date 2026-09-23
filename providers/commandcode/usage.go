package commandcode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hekato-go/config"
	"hekato-go/providers"
)

// Command Code account API (same host as generate; probed 2026-09-23):
//
//	GET /alpha/whoami                → {user:{id,email,name}, org}
//	GET /alpha/billing/credits       → {credits:{monthlyCredits,purchasedCredits,freeCredits},
//	                                    windowLimits:{fiveHour:{used,cap,resetAt}, weekly:{...}}}
//	GET /alpha/billing/subscriptions → {data:{planId,status,currentPeriodEnd}}
//
// Credits are USD. monthlyCredits is what is LEFT of the plan allowance, so
// used = plan allowance - monthlyCredits. Window caps are the CLI's /usage meters.
var usageBase = "https://api.commandcode.ai"

// planCredits mirrors the published plans (commandcode.ai/docs/resources/pricing-limits).
var planCredits = map[string]struct {
	name    string
	credits float64
}{
	"individual-go":       {"Go", 10},
	"individual-goat":     {"GOAT", 70},
	"individual-pro-v1":   {"Pro", 80},
	"individual-pro":      {"Pro", 80},
	"individual-provider": {"Provider", 15},
	"individual-max":      {"Max", 150},
	"individual-ultra":    {"Ultra", 300},
	"teams-pro":           {"Teams Pro", 40},
}

func getJSON(account *config.Account, path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, usageBase+path, nil)
	if err != nil {
		return err
	}
	setHeaders(req, account, false)
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
		return providers.Errorf(resp.StatusCode, "commandcode %s: HTTP %d %s", path, resp.StatusCode, msg)
	}
	return json.Unmarshal(body, out)
}

// FetchUsage reports plan, remaining monthly credits and the 5-hour window.
func FetchUsage(account *config.Account) (*config.AccountInfo, error) {
	if account == nil || strings.TrimSpace(account.AccessToken) == "" {
		return nil, fmt.Errorf("commandcode: account has no API key")
	}
	var who struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
		Org *struct {
			ID string `json:"id"`
		} `json:"org"`
	}
	if err := getJSON(account, "/alpha/whoami", &who); err != nil {
		return nil, err
	}
	q := ""
	if who.Org != nil && who.Org.ID != "" {
		q = "?orgId=" + who.Org.ID
	}
	var credits struct {
		Credits struct {
			Monthly   float64 `json:"monthlyCredits"`
			Purchased float64 `json:"purchasedCredits"`
			Free      float64 `json:"freeCredits"`
		} `json:"credits"`
		Windows struct {
			FiveHour struct {
				Used    float64 `json:"used"`
				Cap     float64 `json:"cap"`
				ResetAt int64   `json:"resetAt"`
			} `json:"fiveHour"`
			Weekly struct {
				Used    float64 `json:"used"`
				Cap     float64 `json:"cap"`
				ResetAt int64   `json:"resetAt"`
			} `json:"weekly"`
		} `json:"windowLimits"`
	}
	if err := getJSON(account, "/alpha/billing/credits"+q, &credits); err != nil {
		return nil, err
	}
	var sub struct {
		Data struct {
			PlanID    string `json:"planId"`
			Status    string `json:"status"`
			PeriodEnd string `json:"currentPeriodEnd"`
		} `json:"data"`
	}
	_ = getJSON(account, "/alpha/billing/subscriptions"+q, &sub)

	info := &config.AccountInfo{
		LastRefresh:       time.Now().Unix(),
		Email:             who.User.Email,
		UserId:            who.User.ID,
		SubscriptionType:  strings.ToUpper(strings.ReplaceAll(sub.Data.PlanID, "individual-", "")),
		SubscriptionTitle: "Command Code",
	}
	plan, known := planCredits[strings.ToLower(sub.Data.PlanID)]
	if known {
		info.SubscriptionTitle = "Command Code " + plan.name
		info.UsageLimit = plan.credits
		info.UsageCurrent = plan.credits - credits.Credits.Monthly
		if info.UsageCurrent < 0 {
			info.UsageCurrent = 0
		}
		if plan.credits > 0 {
			info.UsagePercent = info.UsageCurrent / plan.credits * 100
		}
	} else {
		// Unknown plan: show remaining credits as the "limit" so the bar is meaningful.
		info.UsageLimit = credits.Credits.Monthly + credits.Credits.Purchased + credits.Credits.Free
	}
	if t, err := time.Parse(time.RFC3339, sub.Data.PeriodEnd); err == nil {
		info.NextResetDate = t.Format("2006-01-02")
		info.DaysRemaining = int(time.Until(t).Hours() / 24)
	}
	// 5-hour rolling window as the secondary meter; weekly noted in the status.
	fh := credits.Windows.FiveHour
	info.TrialUsageCurrent = fh.Used
	info.TrialUsageLimit = fh.Cap
	if fh.Cap > 0 {
		info.TrialUsagePercent = fh.Used / fh.Cap * 100
	}
	info.TrialStatus = fmt.Sprintf("5H WINDOW · weekly %.2f/%.0f", credits.Windows.Weekly.Used, credits.Windows.Weekly.Cap)
	if fh.ResetAt > 0 {
		info.TrialExpiresAt = fh.ResetAt / 1000
	}
	return info, nil
}
