// Package ingest receives heartbeats over UDP and feeds the fleet registry.
package ingest

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"stellwerk/control-plane/internal/fleet"
	"stellwerk/control-plane/internal/protocol"
)

// Stats are cumulative counters for the listener.
type Stats struct {
	Received uint64 `json:"received"`
	Accepted uint64 `json:"accepted"`
	Rejected uint64 `json:"rejected"`
}

// Listener reads datagrams from a UDP socket.
type Listener struct {
	conn     net.PacketConn
	registry *fleet.Registry
	log      *slog.Logger
	now      func() time.Time

	received atomic.Uint64
	accepted atomic.Uint64
	rejected atomic.Uint64
}

// Listen binds addr (for example ":7400").
func Listen(addr string, reg *fleet.Registry, log *slog.Logger) (*Listener, error) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}
	return &Listener{conn: conn, registry: reg, log: log, now: time.Now}, nil
}

// Addr is the bound local address.
func (l *Listener) Addr() net.Addr { return l.conn.LocalAddr() }

// Stats returns a snapshot of the counters.
func (l *Listener) Stats() Stats {
	return Stats{Received: l.received.Load(), Accepted: l.accepted.Load(), Rejected: l.rejected.Load()}
}

// Serve blocks until ctx is cancelled or the socket fails.
func (l *Listener) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		l.conn.Close()
	}()
	// One byte more than allowed, so oversize datagrams are detectable.
	buf := make([]byte, protocol.MaxDatagramBytes+1)
	for {
		n, from, err := l.conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		l.handle(buf[:n], from)
	}
}

func (l *Listener) handle(b []byte, from net.Addr) {
	l.received.Add(1)
	hb, err := protocol.Decode(b)
	if err != nil {
		l.rejected.Add(1)
		l.log.Debug("rejected heartbeat", "from", from, "err", err)
		return
	}
	l.accepted.Add(1)
	obs := l.registry.Observe(hb, l.now())
	switch {
	case obs.New:
		l.log.Info("node joined", "node", hb.NodeID, "mode", hb.Mode, "from", from)
	case obs.Restart:
		l.log.Warn("node restarted", "node", hb.NodeID, "boot_id", hb.BootID)
	}
	if obs.Revived {
		l.log.Info("node alive again", "node", hb.NodeID, "missed", obs.Gap)
	}
	if !obs.New && !obs.Stale && obs.ModeFrom != obs.ModeTo {
		level := slog.LevelInfo
		if obs.ModeTo == protocol.ModeMinimalRisk {
			level = slog.LevelWarn
		}
		l.log.Log(context.Background(), level, "mode change", "node", hb.NodeID,
			"from", obs.ModeFrom, "to", obs.ModeTo, "faults", hb.Faults)
	}
}
