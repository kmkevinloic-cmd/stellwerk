package ingest

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"stellwerk/control-plane/internal/fleet"
	"stellwerk/control-plane/internal/protocol"
)

func TestListenerFeedsRegistry(t *testing.T) {
	reg := fleet.NewRegistry(fleet.DefaultConfig())
	l, err := Listen("127.0.0.1:0", reg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- l.Serve(ctx) }()

	conn, err := net.Dial("udp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	good, _ := protocol.Encode(protocol.Heartbeat{
		V: 1, NodeID: "amr-1", BootID: "b", Seq: 0, PeriodMs: 100, Mode: protocol.ModeNominal,
	})
	conn.Write(good)
	conn.Write([]byte("garbage"))
	conn.Write([]byte(`{"v":1,"pad":"` + strings.Repeat("x", 2000) + `"}`))

	deadline := time.Now().Add(2 * time.Second)
	for l.Stats().Received < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	st := l.Stats()
	if st.Received != 3 || st.Accepted != 1 || st.Rejected != 2 {
		t.Fatalf("stats = %+v", st)
	}
	if _, ok := reg.Get("amr-1", time.Now()); !ok {
		t.Fatal("node not registered")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not stop")
	}
}
