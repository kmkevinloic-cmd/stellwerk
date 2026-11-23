// Package fleet keeps the control plane's picture of every node: what each
// node last reported and how fresh that report is. See docs/SAFETY_CONCEPT.md.
package fleet

import (
	"sort"
	"sync"
	"time"

	"stellwerk/control-plane/internal/protocol"
)

// Liveness is derived only from the control plane's own receive times.
type Liveness string

const (
	Alive   Liveness = "ALIVE"
	Suspect Liveness = "SUSPECT"
	Lost    Liveness = "LOST"
)

// Config sets the liveness thresholds as multiples of each node's declared
// heartbeat period.
type Config struct {
	SuspectAfter int // periods of silence before SUSPECT (default 3)
	LostAfter    int // periods of silence before LOST (default 10)
}

// DefaultConfig returns the thresholds documented in the safety concept.
func DefaultConfig() Config { return Config{SuspectAfter: 3, LostAfter: 10} }

// Counters are cumulative per node since it was first seen.
type Counters struct {
	Heartbeats  uint64 `json:"heartbeats"`
	Lost        uint64 `json:"lost"`  // heartbeats inferred missing from sequence gaps
	Stale       uint64 `json:"stale"` // duplicates or reordered heartbeats, ignored
	Restarts    uint64 `json:"restarts"`
	ModeChanges uint64 `json:"mode_changes"`
}

// Node is a point-in-time copy of one node's record.
type Node struct {
	NodeID    string             `json:"node_id"`
	BootID    string             `json:"boot_id"`
	Mode      protocol.Mode      `json:"mode"`
	Faults    []string           `json:"faults"`
	Metrics   map[string]float64 `json:"metrics,omitempty"`
	Liveness  Liveness           `json:"liveness"`
	Usable    bool               `json:"usable"`
	LastSeq   uint64             `json:"last_seq"`
	PeriodMs  int                `json:"period_ms"`
	UptimeMs  uint64             `json:"uptime_ms"`
	FirstSeen time.Time          `json:"first_seen"`
	LastSeen  time.Time          `json:"last_seen"`
	SilentMs  int64              `json:"silent_ms"`
	Counters  Counters           `json:"counters"`
}

// Observation describes what a heartbeat changed.
type Observation struct {
	New      bool   // first heartbeat ever from this node
	Restart  bool   // boot_id changed
	Gap      uint64 // heartbeats inferred lost before this one
	Stale    bool   // duplicate or reordered; the heartbeat was ignored
	ModeFrom protocol.Mode
	ModeTo   protocol.Mode
	Revived  bool // node was SUSPECT or LOST and is ALIVE again
}

// Transition is a liveness change found by Tick.
type Transition struct {
	NodeID string
	From   Liveness
	To     Liveness
}

// Summary aggregates the whole fleet.
type Summary struct {
	Total      int                   `json:"total"`
	Usable     int                   `json:"usable"`
	ByLiveness map[Liveness]int      `json:"by_liveness"`
	ByMode     map[protocol.Mode]int `json:"by_mode"`
}

type record struct {
	hb        protocol.Heartbeat
	liveness  Liveness
	firstSeen time.Time
	lastSeen  time.Time
	counters  Counters
}

// Registry is safe for concurrent use.
type Registry struct {
	cfg   Config
	mu    sync.RWMutex
	nodes map[string]*record
}

// NewRegistry returns an empty registry. Zero or invalid thresholds fall back
// to the defaults.
func NewRegistry(cfg Config) *Registry {
	def := DefaultConfig()
	if cfg.SuspectAfter <= 0 {
		cfg.SuspectAfter = def.SuspectAfter
	}
	if cfg.LostAfter <= cfg.SuspectAfter {
		cfg.LostAfter = max(def.LostAfter, cfg.SuspectAfter+1)
	}
	return &Registry{cfg: cfg, nodes: make(map[string]*record)}
}

// Observe applies one validated heartbeat received at now.
func (r *Registry) Observe(hb protocol.Heartbeat, now time.Time) Observation {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.nodes[hb.NodeID]
	if !ok {
		rec = &record{hb: hb, liveness: Alive, firstSeen: now, lastSeen: now}
		rec.counters.Heartbeats = 1
		r.nodes[hb.NodeID] = rec
		return Observation{New: true, ModeFrom: hb.Mode, ModeTo: hb.Mode}
	}

	obs := Observation{ModeFrom: rec.hb.Mode, ModeTo: hb.Mode}
	switch {
	case hb.BootID != rec.hb.BootID:
		obs.Restart = true
		rec.counters.Restarts++
	case hb.Seq <= rec.hb.Seq:
		// Never let an old heartbeat overwrite newer state.
		obs.Stale = true
		obs.ModeTo = rec.hb.Mode
		rec.counters.Stale++
		return obs
	case hb.Seq > rec.hb.Seq+1:
		obs.Gap = hb.Seq - rec.hb.Seq - 1
		rec.counters.Lost += obs.Gap
	}

	if hb.Mode != rec.hb.Mode {
		rec.counters.ModeChanges++
	}
	obs.Revived = rec.liveness != Alive
	rec.hb = hb
	rec.liveness = Alive
	rec.lastSeen = now
	rec.counters.Heartbeats++
	return obs
}

// Tick re-evaluates liveness for every node at now and returns the changes.
// Call it periodically; silence is only noticed when Tick runs.
func (r *Registry) Tick(now time.Time) []Transition {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []Transition
	for id, rec := range r.nodes {
		next := r.livenessFor(rec, now)
		if next != rec.liveness {
			out = append(out, Transition{NodeID: id, From: rec.liveness, To: next})
			rec.liveness = next
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out
}

func (r *Registry) livenessFor(rec *record, now time.Time) Liveness {
	period := time.Duration(rec.hb.PeriodMs) * time.Millisecond
	silent := now.Sub(rec.lastSeen)
	switch {
	case silent >= time.Duration(r.cfg.LostAfter)*period:
		return Lost
	case silent >= time.Duration(r.cfg.SuspectAfter)*period:
		return Suspect
	default:
		return Alive
	}
}

// Get returns one node.
func (r *Registry) Get(id string, now time.Time) (Node, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.nodes[id]
	if !ok {
		return Node{}, false
	}
	return snapshot(id, rec, now), true
}

// Filter selects nodes in List. Empty fields match everything.
type Filter struct {
	Liveness Liveness
	Mode     protocol.Mode
}

// List returns matching nodes sorted by node ID.
func (r *Registry) List(f Filter, now time.Time) []Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Node, 0, len(r.nodes))
	for id, rec := range r.nodes {
		if f.Liveness != "" && rec.liveness != f.Liveness {
			continue
		}
		if f.Mode != "" && rec.hb.Mode != f.Mode {
			continue
		}
		out = append(out, snapshot(id, rec, now))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out
}

// Summary counts nodes by liveness and by last reported mode.
func (r *Registry) Summary() Summary {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := Summary{
		Total:      len(r.nodes),
		ByLiveness: map[Liveness]int{Alive: 0, Suspect: 0, Lost: 0},
		ByMode: map[protocol.Mode]int{
			protocol.ModeNominal: 0, protocol.ModeDegraded: 0, protocol.ModeMinimalRisk: 0,
		},
	}
	for _, rec := range r.nodes {
		s.ByLiveness[rec.liveness]++
		s.ByMode[rec.hb.Mode]++
		if usable(rec) {
			s.Usable++
		}
	}
	return s
}

// usable: a node may be given work only if it is ALIVE and NOMINAL.
func usable(rec *record) bool {
	return rec.liveness == Alive && rec.hb.Mode == protocol.ModeNominal
}

func snapshot(id string, rec *record, now time.Time) Node {
	faults := append([]string{}, rec.hb.Faults...)
	var metrics map[string]float64
	if len(rec.hb.Metrics) > 0 {
		metrics = make(map[string]float64, len(rec.hb.Metrics))
		for k, v := range rec.hb.Metrics {
			metrics[k] = v
		}
	}
	return Node{
		NodeID:    id,
		BootID:    rec.hb.BootID,
		Mode:      rec.hb.Mode,
		Faults:    faults,
		Metrics:   metrics,
		Liveness:  rec.liveness,
		Usable:    usable(rec),
		LastSeq:   rec.hb.Seq,
		PeriodMs:  rec.hb.PeriodMs,
		UptimeMs:  rec.hb.UptimeMs,
		FirstSeen: rec.firstSeen,
		LastSeen:  rec.lastSeen,
		SilentMs:  now.Sub(rec.lastSeen).Milliseconds(),
		Counters:  rec.counters,
	}
}
