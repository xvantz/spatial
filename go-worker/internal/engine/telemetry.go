package engine

import (
	"log"
	spatialv1 "spatial/gen/spatial/v1"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type TelemetryHandler struct {
	pool       sync.Pool
	batchCount atomic.Uint64
	grid       *SpatialGrid
}

func NewTelemetryHandler() *TelemetryHandler {
	handler := &TelemetryHandler{
		pool: sync.Pool{
			New: func() any {
				return &spatialv1.TelemetryBatch{}
			},
		},
	}

	go handler.mobitorRPS()

	return handler
}

func (h *TelemetryHandler) mobitorRPS() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		count := h.batchCount.Swap(0)
		if count > 0 {
			log.Printf("[Telemetry] Bandwidth: %d batch/sec", count)
		}
	}
}

func (h *TelemetryHandler) HandleNatsMessage(msg *nats.Msg) {
	batch := h.pool.Get().(*spatialv1.TelemetryBatch)

	defer func() {
		batch.Reset()
		h.pool.Put(batch)
	}()

	if err := proto.Unmarshal(msg.Data, batch); err != nil {
		log.Printf("[Telemetry] Error unpacking protobuf: %v", err)
		return
	}

	for _, user := range batch.Players {
		h.grid.UpdatePosition(
			user.UserId,
			user.Position.X,
			user.Position.Y,
			user.Position.Z,
		)
	}

	h.batchCount.Add(1)
}
