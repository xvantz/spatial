// Package main is the entry point for the spatial coprocessor application.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	spatialv1 "spatial/gen/spatial/v1"
	"spatial/internal/engine"
	"spatial/internal/transport"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func main() {
	// Configure structured text logging to stderr.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	slog.Info("starting spatial coprocessor")

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	broker, err := transport.NewBroker(natsURL)
	if err != nil {
		slog.Error("can't start transport", "error", err)
		os.Exit(1)
	}
	defer broker.Shutdown()

	// Root context — cancel triggers graceful shutdown of all goroutines.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	telemetry := engine.NewTelemetryHandler(ctx)
	defer telemetry.Shutdown()

	// Start periodic heartbeat for liveness monitoring.
	telemetry.StartHeartbeat(ctx, broker.NC())

	_, err = broker.Subscribe("spatial.telemetry", telemetry.HandleNatsMessage)
	if err != nil {
		slog.Error("subscribe failed", "subject", "spatial.telemetry", "error", err)
		return
	}
	_, err = broker.Subscribe("spatial.query.visibility", telemetry.HandleVisibilityBatchQuery)
	if err != nil {
		slog.Error("subscribe failed", "subject", "spatial.query.visibility", "error", err)
		return
	}

	_, err = broker.Subscribe("spatial.player.remove", telemetry.HandlePlayerRemove)
	if err != nil {
		slog.Error("subscribe failed", "subject", "spatial.player.remove", "error", err)
		return
	}

	// ---- Handshake: request full state from node-plane ----
	go func() {
		reqData, err := proto.Marshal(&spatialv1.HandshakeRequest{})
		if err != nil {
			slog.Error("handshake marshal failed", "error", err)
			return
		}

		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				slog.Info("handshake aborted", "reason", "shutdown")
				return
			default:
			}

			resp, err := broker.Request("spatial.handshake.sync", reqData, 2*time.Second)
			if err != nil {
				slog.Info("handshake retry", "attempt", i+1, "max", 5)
				time.Sleep(1 * time.Second)
				continue
			}
			telemetry.HandleFullState(resp)
			return
		}
		slog.Warn("handshake failed after 5 retries — continuing without initial state")
	}()

	<-sigChan
	slog.Info("received signal, starting graceful shutdown")

	// Cancel context → all context-aware goroutines (monitorRPS, handshake) exit.
	cancel()

	// Drain NATS connection — wait for in-flight messages to finish.
	broker.Shutdown()

	// telemetry.Shutdown() runs via defer — waits for monitorRPS to finish.
	slog.Info("coprocessor stopped")
}
