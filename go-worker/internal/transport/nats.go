// Package transport handles the messaging infrastructure using NATS.
package transport

import (
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// Broker manages the connection to the NATS message bus.
type Broker struct {
	nc *nats.Conn
}

// NewBroker initializes a new connection to the NATS server.
func NewBroker(url string) (*Broker, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(3),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Printf("[NATS] Disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[NATS] Reconnected: %s", nc.ConnectedUrl())
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("error connection to NATS: %w", err)
	}

	return &Broker{
		nc: nc,
	}, nil
}

// Publish sends a message to the specified subject.
func (b *Broker) Publish(subject string, data []byte) error {
	return b.nc.Publish(subject, data)
}

// Subscribe registers a listener for the specified subject.
func (b *Broker) Subscribe(subject string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return b.nc.Subscribe(subject, handler)
}

// Shutdown gracefully closes the NATS connection.
func (b *Broker) Shutdown() {
	log.Println("[NATS] Start drain...")
	if err := b.nc.Drain(); err != nil {
		log.Printf("[NATS] Error closing connection: %v", err)
	}
	log.Println("[NATS] Connection closed.")
}
