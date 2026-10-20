package proxy

import (
	"encoding/json"
	"hekato-go/config"
	accountpool "hekato-go/pool"
	"net/http/httptest"
	"testing"
)

func TestModelHealthReportsAutoRoutes(t *testing.T) {
	mustInitConfig(t)
	if err := config.AddAccount(config.Account{ID: "mh1", Email: "mh1@x", AccessToken: "t", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("mh1", []string{"claude-sonnet-4.5", "not-in-any-tier"})
	t.Cleanup(func() { p.SetModelList("mh1", nil) })

	h := &Handler{pool: p, autoRouter: newAutoRouter(), modelHealth: newModelHealth()}
	for i := 0; i < 8; i++ {
		h.modelHealth.Record(RequestLog{AccountID: "mh1", Model: "claude-sonnet-4.5", Status: "success", TTFTMs: 2500, TPS: 40})
	}
	for i := 0; i < 2; i++ {
		h.modelHealth.Record(RequestLog{AccountID: "mh1", Model: "claude-sonnet-4.5", Status: "error"})
	}

	rec := httptest.NewRecorder()
	h.apiModelHealth(rec, httptest.NewRequest("GET", "/model-health", nil))
	var got struct {
		Status string        `json:"status"`
		Routes []healthRoute `json:"routes"`
		Days   []healthDay   `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("only the tiered model should be listed, got %+v", got.Routes)
	}
	rt := got.Routes[0]
	// 8/10 = 80% success: degraded.
	if rt.Model != "claude-sonnet-4.5" || rt.Requests != 10 || rt.Errors != 2 || rt.Status != "degraded" || got.Status != "degraded" {
		t.Fatalf("unexpected route %+v (overall %s)", rt, got.Status)
	}
	if rt.TTFTP50 < 2000 || rt.TTFTP50 > 3000 || rt.TPS != 40 {
		t.Fatalf("ttft p50 %d / tps %v off", rt.TTFTP50, rt.TPS)
	}
	if len(got.Days) != healthDays || got.Days[healthDays-1].Requests != 10 || rt.Days[healthDays-1] != "degraded" || rt.Days[0] != "none" {
		t.Fatalf("daily strip wrong: %+v", got.Days[healthDays-1])
	}
}
