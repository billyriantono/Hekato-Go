package proxy

import (
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/logger"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Update check: polls the latest GitHub release so the dashboard can tell the
// operator a newer version is out. Check only; applying it stays manual.

const (
	latestReleaseURL = "https://api.github.com/repos/billyriantono/Hekato-Go/releases/latest"
	updateCheckEvery = 6 * time.Hour
)

type updateInfo struct {
	Latest    string `json:"latest,omitempty"`
	URL       string `json:"url,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Available bool   `json:"available"`
	CheckedAt int64  `json:"checkedAt,omitempty"`
}

var latestUpdate atomic.Pointer[updateInfo]

func runUpdateChecker() {
	for {
		if err := checkForUpdate(); err != nil {
			logger.Warnf("[UpdateCheck] %v", err)
		}
		time.Sleep(updateCheckEvery)
	}
}

func checkForUpdate() error {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", latestReleaseURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "hekato-go/"+config.Version)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("latest release: HTTP %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	info := &updateInfo{
		Latest:    latest,
		URL:       rel.HTMLURL,
		Notes:     rel.Body,
		Available: versionNewer(latest, config.Version),
		CheckedAt: time.Now().Unix(),
	}
	latestUpdate.Store(info)
	if info.Available {
		logger.Infof("[UpdateCheck] v%s is available (running %s): %s", latest, config.Version, rel.HTMLURL)
	}
	return nil
}

// versionNewer reports whether release a is newer than the running build b.
// Only major.minor.patch counts: a build past a tag ("1.1.3-4-gabc1234")
// is not behind release 1.1.3. Unparsable versions (dev builds) never are.
func versionNewer(a, b string) bool {
	pa, okA := parseVersion(a)
	pb, okB := parseVersion(b)
	if !okA || !okB {
		return false
	}
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	core, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(v), "v"), "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
