// Package main is the entry point for the spatial coprocessor application.
package main

import (
	"log"
	"os"
	"os/signal"
	"spatial/internal/engine"
	"spatial/internal/transport"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	log.Println("[Bootstrap] Start Spatial Coprocessor...")

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	broker, err := transport.NewBroker(natsURL)
	if err != nil {
		log.Fatalf("[Fatal] Can`t start transport: %v", err)
	}

	defer broker.Shutdown()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	telemetry := engine.NewTelemetryHandler()

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

	<-sigChan
	log.Println("\n[Shutdown] Get signal. Start closing...")

	time.Sleep(100 * time.Millisecond)
	log.Println("[Shutdown] Coprocessor stopped.")
}
