// Package api serves a read-only HTTP/JSON view of the fleet registry.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"stellwerk/control-plane/internal/fleet"
	"stellwerk/control-plane/internal/ingest"
	"stellwerk/control-plane/internal/protocol"
)

// StatsFunc reports ingest counters; it may be nil.
type StatsFunc func() ingest.Stats

// Handler builds the HTTP routes:
//
//	GET /healthz
//	GET /v1/fleet/summary
//	GET /v1/nodes?liveness=LOST&mode=DEGRADED
//	GET /v1/nodes/{id}
func Handler(reg *fleet.Registry, stats StatsFunc, now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /v1/fleet/summary", func(w http.ResponseWriter, _ *http.Request) {
		body := struct {
			fleet.Summary
			Ingest *ingest.Stats `json:"ingest,omitempty"`
		}{Summary: reg.Summary()}
		if stats != nil {
			s := stats()
			body.Ingest = &s
		}
		writeJSON(w, http.StatusOK, body)
	})

	mux.HandleFunc("GET /v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		f := fleet.Filter{
			Liveness: fleet.Liveness(r.URL.Query().Get("liveness")),
			Mode:     protocol.Mode(r.URL.Query().Get("mode")),
		}
		if f.Liveness != "" && f.Liveness != fleet.Alive && f.Liveness != fleet.Suspect && f.Liveness != fleet.Lost {
			writeError(w, http.StatusBadRequest, "liveness must be ALIVE, SUSPECT or LOST")
			return
		}
		if f.Mode != "" && !f.Mode.Valid() {
			writeError(w, http.StatusBadRequest, "mode must be NOMINAL, DEGRADED or MINIMAL_RISK")
			return
		}
		nodes := reg.List(f, now())
		writeJSON(w, http.StatusOK, map[string]any{"count": len(nodes), "nodes": nodes})
	})

	mux.HandleFunc("GET /v1/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		n, ok := reg.Get(r.PathValue("id"), now())
		if !ok {
			writeError(w, http.StatusNotFound, "unknown node")
			return
		}
		writeJSON(w, http.StatusOK, n)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
