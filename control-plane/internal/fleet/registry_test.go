package fleet

import (
	"testing"
	"time"

	"stellwerk/control-plane/internal/protocol"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func hb(id, boot string, seq uint64, mode protocol.Mode) protocol.Heartbeat {
	return protocol.Heartbeat{V: 1, NodeID: id, BootID: boot, Seq: seq, PeriodMs: 100, Mode: mode}
}

func at(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }

func TestFirstHeartbeatRegistersAliveNode(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	obs := r.Observe(hb("a", "b1", 0, protocol.ModeNominal), at(0))
	if !obs.New {
		t.Fatal("expected New")
	}
	n, ok := r.Get("a", at(0))
	if !ok || n.Liveness != Alive || !n.Usable || n.Counters.Heartbeats != 1 {
		t.Fatalf("unexpected node: %+v", n)
	}
}

func TestSequenceGapCountsLostHeartbeats(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	r.Observe(hb("a", "b1", 0, protocol.ModeNominal), at(0))
	obs := r.Observe(hb("a", "b1", 4, protocol.ModeNominal), at(400))
	if obs.Gap != 3 {
		t.Fatalf("gap = %d, want 3", obs.Gap)
	}
	n, _ := r.Get("a", at(400))
	if n.Counters.Lost != 3 || n.LastSeq != 4 {
		t.Fatalf("counters = %+v", n.Counters)
	}
}

func TestStaleHeartbeatIsIgnored(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	r.Observe(hb("a", "b1", 5, protocol.ModeMinimalRisk), at(0))
	obs := r.Observe(hb("a", "b1", 3, protocol.ModeNominal), at(10))
	if !obs.Stale {
		t.Fatal("expected Stale")
	}
	n, _ := r.Get("a", at(10))
	if n.Mode != protocol.ModeMinimalRisk {
		t.Fatalf("stale heartbeat overwrote mode: %s", n.Mode)
	}
	if n.LastSeen != at(0) {
		t.Fatal("stale heartbeat refreshed liveness")
	}
	if n.Counters.Stale != 1 {
		t.Fatalf("stale counter = %d", n.Counters.Stale)
	}
}

func TestRestartIsDetectedEvenWithLowerSeq(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	r.Observe(hb("a", "b1", 900, protocol.ModeNominal), at(0))
	obs := r.Observe(hb("a", "b2", 0, protocol.ModeNominal), at(50))
	if !obs.Restart || obs.Stale || obs.Gap != 0 {
		t.Fatalf("unexpected observation: %+v", obs)
	}
	n, _ := r.Get("a", at(50))
	if n.BootID != "b2" || n.LastSeq != 0 || n.Counters.Restarts != 1 {
		t.Fatalf("unexpected node: %+v", n)
	}
}

func TestLivenessProgressesWithSilence(t *testing.T) {
	r := NewRegistry(DefaultConfig()) // suspect 3×, lost 10× of 100 ms
	r.Observe(hb("a", "b1", 0, protocol.ModeNominal), at(0))

	if tr := r.Tick(at(299)); len(tr) != 0 {
		t.Fatalf("too early: %+v", tr)
	}
	tr := r.Tick(at(300))
	if len(tr) != 1 || tr[0].From != Alive || tr[0].To != Suspect {
		t.Fatalf("want ALIVE->SUSPECT, got %+v", tr)
	}
	if n, _ := r.Get("a", at(300)); n.Usable {
		t.Fatal("SUSPECT node must not be usable")
	}
	tr = r.Tick(at(1000))
	if len(tr) != 1 || tr[0].To != Lost {
		t.Fatalf("want SUSPECT->LOST, got %+v", tr)
	}
	if tr := r.Tick(at(5000)); len(tr) != 0 {
		t.Fatalf("LOST should be stable, got %+v", tr)
	}

	obs := r.Observe(hb("a", "b1", 50, protocol.ModeNominal), at(5000))
	if !obs.Revived || obs.Gap != 49 {
		t.Fatalf("unexpected revival: %+v", obs)
	}
	if n, _ := r.Get("a", at(5000)); n.Liveness != Alive {
		t.Fatalf("liveness = %s", n.Liveness)
	}
}

func TestLivenessJumpsStraightToLost(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	r.Observe(hb("a", "b1", 0, protocol.ModeNominal), at(0))
	tr := r.Tick(at(2000))
	if len(tr) != 1 || tr[0].From != Alive || tr[0].To != Lost {
		t.Fatalf("got %+v", tr)
	}
}

func TestUsableRequiresAliveAndNominal(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	r.Observe(hb("n", "b", 0, protocol.ModeNominal), at(0))
	r.Observe(hb("d", "b", 0, protocol.ModeDegraded), at(0))
	r.Observe(hb("m", "b", 0, protocol.ModeMinimalRisk), at(0))
	s := r.Summary()
	if s.Total != 3 || s.Usable != 1 || s.ByMode[protocol.ModeDegraded] != 1 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestListFilterAndOrder(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	r.Observe(hb("c", "b", 0, protocol.ModeNominal), at(0))
	r.Observe(hb("a", "b", 0, protocol.ModeDegraded), at(0))
	r.Observe(hb("b", "b", 0, protocol.ModeDegraded), at(500))
	r.Tick(at(500)) // a and c become SUSPECT

	all := r.List(Filter{}, at(500))
	if len(all) != 3 || all[0].NodeID != "a" || all[2].NodeID != "c" {
		t.Fatalf("order: %+v", all)
	}
	got := r.List(Filter{Liveness: Suspect, Mode: protocol.ModeDegraded}, at(500))
	if len(got) != 1 || got[0].NodeID != "a" {
		t.Fatalf("filter: %+v", got)
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	h := hb("a", "b", 0, protocol.ModeDegraded)
	h.Faults = []string{"f1"}
	h.Metrics = map[string]float64{"x": 1}
	r.Observe(h, at(0))
	n, _ := r.Get("a", at(0))
	n.Faults[0] = "mutated"
	n.Metrics["x"] = 99
	again, _ := r.Get("a", at(0))
	if again.Faults[0] != "f1" || again.Metrics["x"] != 1 {
		t.Fatal("snapshot aliases registry state")
	}
}

func TestInvalidConfigFallsBack(t *testing.T) {
	r := NewRegistry(Config{SuspectAfter: 5, LostAfter: 2})
	if r.cfg.SuspectAfter != 5 || r.cfg.LostAfter <= 5 {
		t.Fatalf("cfg = %+v", r.cfg)
	}
}
