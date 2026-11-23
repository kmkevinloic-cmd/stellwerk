package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stellwerk/control-plane/internal/fleet"
	"stellwerk/control-plane/internal/ingest"
	"stellwerk/control-plane/internal/protocol"
)

func setup() http.Handler {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reg := fleet.NewRegistry(fleet.DefaultConfig())
	reg.Observe(protocol.Heartbeat{V: 1, NodeID: "a", BootID: "b", PeriodMs: 100, Mode: protocol.ModeNominal}, t0)
	reg.Observe(protocol.Heartbeat{V: 1, NodeID: "b", BootID: "b", PeriodMs: 100, Mode: protocol.ModeDegraded,
		Faults: []string{"infer_deadline"}}, t0)
	stats := func() ingest.Stats { return ingest.Stats{Received: 2, Accepted: 2} }
	return Handler(reg, stats, func() time.Time { return t0.Add(50 * time.Millisecond) })
}

func get(t *testing.T, h http.Handler, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: invalid JSON: %v", path, err)
	}
	return rec.Code, body
}

func TestSummary(t *testing.T) {
	code, body := get(t, setup(), "/v1/fleet/summary")
	if code != 200 || body["total"].(float64) != 2 || body["usable"].(float64) != 1 {
		t.Fatalf("%d %v", code, body)
	}
	if body["ingest"].(map[string]any)["accepted"].(float64) != 2 {
		t.Fatalf("ingest stats missing: %v", body)
	}
}

func TestListAndFilter(t *testing.T) {
	h := setup()
	_, body := get(t, h, "/v1/nodes")
	if body["count"].(float64) != 2 {
		t.Fatalf("%v", body)
	}
	_, body = get(t, h, "/v1/nodes?mode=DEGRADED")
	nodes := body["nodes"].([]any)
	if len(nodes) != 1 || nodes[0].(map[string]any)["node_id"] != "b" {
		t.Fatalf("%v", body)
	}
	if code, _ := get(t, h, "/v1/nodes?liveness=dead"); code != 400 {
		t.Fatalf("want 400, got %d", code)
	}
	if code, _ := get(t, h, "/v1/nodes?mode=PANIC"); code != 400 {
		t.Fatalf("want 400, got %d", code)
	}
}

func TestGetNode(t *testing.T) {
	h := setup()
	code, body := get(t, h, "/v1/nodes/b")
	if code != 200 || body["mode"] != "DEGRADED" || body["silent_ms"].(float64) != 50 {
		t.Fatalf("%d %v", code, body)
	}
	if code, _ := get(t, h, "/v1/nodes/zzz"); code != 404 {
		t.Fatalf("want 404, got %d", code)
	}
}

func TestHealthz(t *testing.T) {
	if code, body := get(t, setup(), "/healthz"); code != 200 || body["status"] != "ok" {
		t.Fatalf("%d %v", code, body)
	}
}
