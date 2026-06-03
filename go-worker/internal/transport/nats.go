// Package transport handles the messaging infrastructure using NATS.
package transport

import (
	"fmt"
	"log/slog"
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
			slog.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("nats reconnected", "url", nc.ConnectedUrl())
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

// Request sends a request and waits for a reply on an auto-managed inbox.
func (b *Broker) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	return b.nc.Request(subject, data, timeout)
}

// Subscribe registers a listener for the specified subject.
func (b *Broker) Subscribe(subject string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return b.nc.Subscribe(subject, handler)
}

// NC returns the underlying NATS connection for direct publish access.
func (b *Broker) NC() *nats.Conn {
	return b.nc
}

// Shutdown gracefully closes the NATS connection.
func (b *Broker) Shutdown() {
	slog.Info("nats drain starting")
	if err := b.nc.Drain(); err != nil {
		slog.Error("nats drain failed", "error", err)
	}
	slog.Info("nats connection closed")
}
