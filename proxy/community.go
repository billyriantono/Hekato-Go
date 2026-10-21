package proxy

import (
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

// communityLogsMax caps how many recent requests the public page lists.
const communityLogsMax = 100

// communityKey returns the API key published on the public /bansos page,
// named by the COMMUNITY_KEY_ID env var (an api-key row id). nil means no
// event is running: the env var is unset, the key is gone, or it's disabled.
// Quota, RPM, concurrency and model allowlist come from the key itself, so
// the event is configured and ended from the normal API Keys admin page.
func communityKey() *config.ApiKeyEntry {
	id := os.Getenv("COMMUNITY_KEY_ID")
	if id == "" {
		return nil
	}
	e := config.GetApiKeyEntry(id)
	if e == nil || !e.Enabled {
		return nil
	}
	return e
}

// handleCommunity powers the public community-event page (/bansos): it hands out the
// shared key with its quota and recent traffic, no login needed. Everyone sees
// everyone's requests here, so client IPs are masked and upstream account ids
// dropped.
func (h *Handler) handleCommunity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	entry := communityKey()
	if entry == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"active": false})
		return
	}
	logs := make([]RequestLog, 0, communityLogsMax)
	for _, log := range h.getRequestLogs() { // newest-first
		if log.ApiKeyID != entry.ID {
			continue
		}
		log.AccountID = ""
		log.ClientIP = maskIP(log.ClientIP)
		logs = append(logs, log)
		if len(logs) == communityLogsMax {
			break
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active":           true,
		"name":             entry.Name,
		"key":              entry.Key,
		"tokensUsed":       entry.TokensUsed,
		"tokenLimit":       entry.TokenLimit,
		"requestsCount":    entry.RequestsCount,
		"rpmLimit":         entry.RPMLimit,
		"concurrencyLimit": entry.ConcurrencyLimit,
		"allowedModels":    entry.AllowedModels,
		"logs":             logs,
	})
}

// maskIP keeps only the network half of an address: "203.0.113.7" →
// "203.0.x.x", "2001:db8:1::5" → "2001:db8:…". Unparseable input → "".
func maskIP(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	addr = addr.Unmap()
	if addr.Is4() {
		b := addr.As4()
		return fmt.Sprintf("%d.%d.x.x", b[0], b[1])
	}
	parts := strings.SplitN(addr.StringExpanded(), ":", 3)
	return strings.TrimLeft(parts[0], "0") + ":" + strings.TrimLeft(parts[1], "0") + ":…"
}
