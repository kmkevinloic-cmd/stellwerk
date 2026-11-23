// Package protocol defines the Stellwerk heartbeat message (v1) and its
// validation rules. See docs/PROTOCOL.md.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Version is the only protocol version this package accepts.
const Version = 1

// Limits from docs/PROTOCOL.md.
const (
	MaxDatagramBytes = 1400
	MaxNodeIDLen     = 64
	MaxBootIDLen     = 32
	MaxFaults        = 32
	MaxMetrics       = 32
	MinPeriodMs      = 10
	MaxPeriodMs      = 60000
)

// Mode is a node's self-reported operating mode.
type Mode string

const (
	ModeNominal     Mode = "NOMINAL"
	ModeDegraded    Mode = "DEGRADED"
	ModeMinimalRisk Mode = "MINIMAL_RISK"
)

// Valid reports whether m is one of the defined modes.
func (m Mode) Valid() bool {
	switch m {
	case ModeNominal, ModeDegraded, ModeMinimalRisk:
		return true
	}
	return false
}

// Heartbeat is one decoded and validated heartbeat.
type Heartbeat struct {
	V        int                `json:"v"`
	NodeID   string             `json:"node_id"`
	BootID   string             `json:"boot_id"`
	Seq      uint64             `json:"seq"`
	PeriodMs int                `json:"period_ms"`
	UptimeMs uint64             `json:"uptime_ms"`
	Mode     Mode               `json:"mode"`
	Faults   []string           `json:"faults,omitempty"`
	Metrics  map[string]float64 `json:"metrics,omitempty"`
}

// ErrTooLarge is returned for datagrams above MaxDatagramBytes.
var ErrTooLarge = errors.New("heartbeat exceeds maximum datagram size")

// Decode parses and validates a single heartbeat datagram.
func Decode(b []byte) (Heartbeat, error) {
	var hb Heartbeat
	if len(b) > MaxDatagramBytes {
		return hb, ErrTooLarge
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&hb); err != nil {
		return hb, fmt.Errorf("decode: %w", err)
	}
	if dec.More() {
		return hb, errors.New("decode: trailing data after heartbeat object")
	}
	if err := hb.Validate(); err != nil {
		return hb, err
	}
	return hb, nil
}

// Encode serialises a heartbeat. It is used by tests and tools; nodes in
// other languages produce the same JSON.
func Encode(hb Heartbeat) ([]byte, error) {
	if err := hb.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(hb)
	if err != nil {
		return nil, err
	}
	if len(b) > MaxDatagramBytes {
		return nil, ErrTooLarge
	}
	return b, nil
}

// Validate checks every field against the v1 rules.
func (hb Heartbeat) Validate() error {
	if hb.V != Version {
		return fmt.Errorf("unsupported protocol version %d", hb.V)
	}
	if !validID(hb.NodeID, MaxNodeIDLen) {
		return fmt.Errorf("invalid node_id %q", hb.NodeID)
	}
	if len(hb.BootID) == 0 || len(hb.BootID) > MaxBootIDLen {
		return fmt.Errorf("invalid boot_id length %d", len(hb.BootID))
	}
	if hb.PeriodMs < MinPeriodMs || hb.PeriodMs > MaxPeriodMs {
		return fmt.Errorf("period_ms %d out of range [%d, %d]", hb.PeriodMs, MinPeriodMs, MaxPeriodMs)
	}
	if !hb.Mode.Valid() {
		return fmt.Errorf("invalid mode %q", hb.Mode)
	}
	if len(hb.Faults) > MaxFaults {
		return fmt.Errorf("too many faults (%d > %d)", len(hb.Faults), MaxFaults)
	}
	for _, f := range hb.Faults {
		if !validID(f, MaxNodeIDLen) {
			return fmt.Errorf("invalid fault name %q", f)
		}
	}
	if len(hb.Metrics) > MaxMetrics {
		return fmt.Errorf("too many metrics (%d > %d)", len(hb.Metrics), MaxMetrics)
	}
	for k := range hb.Metrics {
		if !validID(k, MaxNodeIDLen) {
			return fmt.Errorf("invalid metric name %q", k)
		}
	}
	return nil
}

// validID accepts 1..max characters from [A-Za-z0-9._-].
func validID(s string, max int) bool {
	if len(s) == 0 || len(s) > max {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}
