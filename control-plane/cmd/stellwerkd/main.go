// Command stellwerkd is the Stellwerk fleet control plane.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stellwerk/control-plane/internal/api"
	"stellwerk/control-plane/internal/fleet"
	"stellwerk/control-plane/internal/ingest"
)

// version is the release version. The Makefile and release workflow set it
// from the repository's VERSION file with -ldflags "-X main.version=...".
var version = "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stellwerkd:", err)
		os.Exit(1)
	}
}

func run() error {
	udpAddr := flag.String("udp", ":7400", "UDP address for heartbeats")
	httpAddr := flag.String("http", ":7401", "HTTP address for the API")
	tick := flag.Duration("tick", 50*time.Millisecond, "liveness evaluation interval")
	suspect := flag.Int("suspect-after", 3, "silent periods before a node is SUSPECT")
	lost := flag.Int("lost-after", 10, "silent periods before a node is LOST")
	debug := flag.Bool("debug", false, "log rejected heartbeats")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("stellwerkd", version)
		return nil
	}

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	reg := fleet.NewRegistry(fleet.Config{SuspectAfter: *suspect, LostAfter: *lost})

	listener, err := ingest.Listen(*udpAddr, reg, log)
	if err != nil {
		return fmt.Errorf("udp listen: %w", err)
	}
	srv := &http.Server{
		Addr:              *httpAddr,
		Handler:           api.Handler(reg, listener.Stats, nil),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 2)
	go func() { errc <- listener.Serve(ctx) }()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- fmt.Errorf("http: %w", err)
		}
	}()
	go superviseLiveness(ctx, reg, *tick, log)

	log.Info("stellwerkd started", "version", version, "udp", listener.Addr(), "http", *httpAddr)

	select {
	case <-ctx.Done():
	case err := <-errc:
		if err != nil {
			stop()
			return err
		}
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func superviseLiveness(ctx context.Context, reg *fleet.Registry, every time.Duration, log *slog.Logger) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			for _, tr := range reg.Tick(now) {
				level := slog.LevelInfo
				if tr.To == fleet.Lost {
					level = slog.LevelWarn
				}
				log.Log(ctx, level, "liveness change", "node", tr.NodeID, "from", tr.From, "to", tr.To)
			}
		}
	}
}
