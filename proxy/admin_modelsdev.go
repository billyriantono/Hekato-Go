package proxy

import (
	"encoding/json"
	"hekato-go/providers/modelsdev"
	"net/http"
)

// apiGetModelsDev: GET /admin/api/models-dev[?refresh=1]
// Serves the cached models.dev catalog (pricing, limits, capabilities) for the
// admin panel's Model Prices page.
func (h *Handler) apiGetModelsDev(w http.ResponseWriter, r *http.Request) {
	list, at, err := modelsdev.List()
	if r.URL.Query().Get("refresh") == "1" {
		list, at, err = modelsdev.Refresh()
	}
	if err != nil {
		w.WriteHeader(502)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models":    list,
		"fetchedAt": at.Unix(),
		"source":    "https://models.dev/api.json",
	})
}
