package opencodego

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

// usageURL is a var so tests can point it at a stub; production is goUsageURL.
var usageURL = goUsageURL

type goWindow struct {
	Status   string  `json:"status"` // ok | rate-limited
	Percent  float64 `json:"percent"`
	ResetsAt string  `json:"resetsAt"`
}

// FetchUsage reads GET /zen/go/v1/usage: {usage:{rolling,weekly,monthly}}
// where each window carries a percent used and a reset time (no absolute
// token numbers are exposed). Rolling (5h) is the main meter, weekly the
// secondary one, monthly sets the reset date.
func FetchUsage(account *config.Account) (*config.AccountInfo, error) {
	if account == nil || strings.TrimSpace(account.AccessToken) == "" {
		return nil, fmt.Errorf("opencodego: account has no API key")
	}
	req, err := http.NewRequest(http.MethodGet, usageURL, nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req, account)
	req.Header.Set("Accept", "application/json")
	resp, err := providers.GetRestClientForAccount(account).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, providers.Errorf(resp.StatusCode, "opencodego usage: HTTP %d %s", resp.StatusCode, msg)
	}
	var out struct {
		Usage struct {
			Rolling goWindow `json:"rolling"`
			Weekly  goWindow `json:"weekly"`
			Monthly goWindow `json:"monthly"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("opencodego usage: %w", err)
	}
	u := out.Usage
	info := &config.AccountInfo{
		LastRefresh:       time.Now().Unix(),
		SubscriptionType:  "GO",
		SubscriptionTitle: "OpenCode Go",
		UsageCurrent:      u.Rolling.Percent,
		UsageLimit:        100,
		UsagePercent:      u.Rolling.Percent,
		TrialUsageCurrent: u.Weekly.Percent,
		TrialUsageLimit:   100,
		TrialUsagePercent: u.Weekly.Percent,
		TrialStatus:       "WEEKLY " + strings.ToUpper(u.Weekly.Status),
	}
	if t, err := time.Parse(time.RFC3339, u.Rolling.ResetsAt); err == nil {
		info.NextResetDate = t.Local().Format("2006-01-02 15:04")
	}
	if t, err := time.Parse(time.RFC3339, u.Weekly.ResetsAt); err == nil {
		info.TrialExpiresAt = t.Unix()
	}
	if t, err := time.Parse(time.RFC3339, u.Monthly.ResetsAt); err == nil {
		info.DaysRemaining = int(time.Until(t).Hours() / 24)
	}
	return info, nil
}
