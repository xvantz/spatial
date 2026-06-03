// Package main is the entry point for the spatial coprocessor application.
package main

import (
	"context"
	"log"
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
	log.Println("[Bootstrap] Start Spatial Coprocessor...")

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	broker, err := transport.NewBroker(natsURL)
	if err != nil {
		log.Fatalf("[Fatal] Can't start transport: %v", err)
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
		log.Printf("[Fatal] Error subscribe to spatial.telemetry: %v", err)
		return
	}
	_, err = broker.Subscribe("spatial.query.visibility", telemetry.HandleVisibilityBatchQuery)
	if err != nil {
		log.Printf("[Fatal] Error subscribe to spatial.query.visibility: %v", err)
		return
	}

	_, err = broker.Subscribe("spatial.player.remove", telemetry.HandlePlayerRemove)
	if err != nil {
		log.Printf("[Fatal] Error subscribe to spatial.player.remove: %v", err)
		return
	}

	// ---- Handshake: request full state from node-plane ----
	go func() {
		reqData, err := proto.Marshal(&spatialv1.HandshakeRequest{})
		if err != nil {
			log.Printf("[Handshake] Failed to marshal request: %v", err)
			return
		}

		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				log.Println("[Handshake] Shutdown requested, aborting handshake")
				return
			default:
			}

			resp, err := broker.Request("spatial.handshake.sync", reqData, 2*time.Second)
			if err != nil {
				log.Printf("[Handshake] Waiting for node-plane... (attempt %d/5)", i+1)
				time.Sleep(1 * time.Second)
				continue
			}
			telemetry.HandleFullState(resp)
			return
		}
		log.Println("[Handshake] Failed after 5 retries — continuing without initial state")
	}()

	<-sigChan
	log.Println("\n[Shutdown] Get signal. Starting graceful shutdown...")

	// Cancel context → all context-aware goroutines (monitorRPS, handshake) exit.
	cancel()

	// Drain NATS connection — wait for in-flight messages to finish.
	broker.Shutdown()

	// telemetry.Shutdown() runs via defer — waits for monitorRPS to finish.
	log.Println("[Shutdown] Coprocessor stopped.")
}
