package engine

import (
	"context"
	"log"
	"spatial/internal/transport"
	"time"
)

type Metronome struct {
	broker *transport.Broker
	ticker *time.Ticker
}

func NewMetronome(b *transport.Broker) *Metronome {
	return &Metronome{
		broker: b,
	}
}

func (m *Metronome) Start(ctx context.Context, tickRate time.Duration) {
	m.ticker = time.NewTicker(tickRate)

	log.Printf("[Metronome] Start metronome. Rate: %v", tickRate)

	go func() {
		defer m.ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Printf("[Metronome] stopped by context")
				return
			case t := <-m.ticker.C:
				payload := []byte(t.String())
				if err := m.broker.Publish("engine.tick", payload); err != nil {
					log.Printf("[Metronome] Error send tick")
				}
			}
		}
	}()
}
