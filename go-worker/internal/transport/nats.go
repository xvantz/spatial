package transport

import (
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

type Broker struct {
	nc *nats.Conn
}

func NewBroker(url string) (*Broker, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(3),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
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

func (b *Broker) Publish(subject string, data []byte) error {
	return b.nc.Publish(subject, data)
}

func (b *Broker) Subscribe(subject string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return b.nc.Subscribe(subject, handler)
}

func (b *Broker) Shutdown() {
	log.Println("[NATS] Start drain...")
	if err := b.nc.Drain(); err != nil {
		log.Printf("[NATS] Error closing connection: %v", err)
	}
	log.Println("[NATS] Connection closed.")
}
