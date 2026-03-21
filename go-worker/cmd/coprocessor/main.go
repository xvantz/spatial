package main

import (
	"context"
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

	broker, err := transport.NewBroker(nats.DefaultURL)
	if err != nil {
		log.Fatalf("[Fatal] Can`t start transport: %v", err)
	}

	defer broker.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	_, err = broker.Subscribe("state.update", func(msg *nats.Msg) {

	})
	if err != nil {
		log.Fatalf("[Fatal] Error subscribe to state.update: %v", err)
	}

	metronome := engine.NewMetronome(broker)
	metronome.Start(ctx, 16*time.Millisecond)

	<-sigChan
	log.Println("\n[Shutdown] Get signal. Start closing...")

	cancel()

	time.Sleep(100 * time.Millisecond)
	log.Println("[Shutdown] Coprocessor stopped.")
}
