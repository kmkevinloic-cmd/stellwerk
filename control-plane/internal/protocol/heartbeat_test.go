package protocol

import (
	"strings"
	"testing"
)

func valid() Heartbeat {
	return Heartbeat{
		V: 1, NodeID: "amr-0042", BootID: "9f1c2a7e", Seq: 7, PeriodMs: 100,
		UptimeMs: 700, Mode: ModeDegraded, Faults: []string{"infer_deadline"},
		Metrics: map[string]float64{"battery": 0.6},
	}
}

func TestRoundTrip(t *testing.T) {
	b, err := Encode(valid())
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.NodeID != "amr-0042" || got.Seq != 7 || got.Mode != ModeDegraded || got.Metrics["battery"] != 0.6 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestDecodeIgnoresUnknownFields(t *testing.T) {
	raw := `{"v":1,"node_id":"n1","boot_id":"b","seq":0,"period_ms":50,"uptime_ms":0,"mode":"NOMINAL","future":true}`
	if _, err := Decode([]byte(raw)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidationRejects(t *testing.T) {
	cases := map[string]func(*Heartbeat){
		"version":       func(h *Heartbeat) { h.V = 2 },
		"empty node":    func(h *Heartbeat) { h.NodeID = "" },
		"bad node char": func(h *Heartbeat) { h.NodeID = "amr 1" },
		"long node":     func(h *Heartbeat) { h.NodeID = strings.Repeat("a", 65) },
		"empty boot":    func(h *Heartbeat) { h.BootID = "" },
		"long boot":     func(h *Heartbeat) { h.BootID = strings.Repeat("b", 33) },
		"period low":    func(h *Heartbeat) { h.PeriodMs = 9 },
		"period high":   func(h *Heartbeat) { h.PeriodMs = 60001 },
		"mode":          func(h *Heartbeat) { h.Mode = "PANIC" },
		"fault name":    func(h *Heartbeat) { h.Faults = []string{"bad name"} },
		"too many faults": func(h *Heartbeat) {
			h.Faults = make([]string, MaxFaults+1)
			for i := range h.Faults {
				h.Faults[i] = "f"
			}
		},
		"metric name": func(h *Heartbeat) { h.Metrics = map[string]float64{"": 1} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			hb := valid()
			mutate(&hb)
			if err := hb.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	for name, raw := range map[string]string{
		"not json":  "hello",
		"trailing":  `{"v":1,"node_id":"n","boot_id":"b","seq":0,"period_ms":50,"mode":"NOMINAL"} {}`,
		"neg seq":   `{"v":1,"node_id":"n","boot_id":"b","seq":-1,"period_ms":50,"mode":"NOMINAL"}`,
		"too large": `{"v":1,"pad":"` + strings.Repeat("x", MaxDatagramBytes) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(raw)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
