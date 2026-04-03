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

// TelemetryHandler handles incoming spatial updates and visibility queries.
// It manages a high-performance SpatialGrid and uses object pooling to reduce GC pressure.
type TelemetryHandler struct {
	// pool provides reusable protobuf message objects to minimize allocations
	// during high-frequency telemetry processing.
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
		grid: NewSpatialGrid(),
	}

	go handler.monitorRPS()

	return handler
}

func (h *TelemetryHandler) monitorRPS() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		count := h.batchCount.Swap(0)
		if count > 0 {
			log.Printf("[Telemetry] Processing rate: %d batches/sec", count/10)
		}
	}
}

// HandleNatsMessage processes incoming telemetry batches and maintains world state hash.
func (h *TelemetryHandler) HandleNatsMessage(msg *nats.Msg) {
	// Acquire a message object from the pool.
	batch := h.pool.Get().(*spatialv1.TelemetryBatch)

	defer func() {
		// Reset state and return the object to the pool for reuse.
		batch.Reset()
		h.pool.Put(batch)
	}()

	if err := proto.Unmarshal(msg.Data, batch); err != nil {
		return
	}

	// Atomically update world state and hash.
	h.grid.BulkUpdate(batch.Players)

	// Log desynchronization if detected (optional, but useful for monitoring).
	if batch.StateHash != 0 && batch.StateHash != h.grid.GetTotalHash() {
		log.Printf("[Sync] Desync detected. Remote: %v | Local: %v", batch.StateHash, h.grid.GetTotalHash())
	}

	h.batchCount.Add(1)
}

// HandleVisibilityBatchQuery processes visibility range requests from clients.
func (h *TelemetryHandler) HandleVisibilityBatchQuery(msg *nats.Msg) {
	if msg.Reply == "" {
		return
	}

	batchQuery := &spatialv1.VisibilityBatchQuery{}
	if err := proto.Unmarshal(msg.Data, batchQuery); err != nil {
		return
	}

	results := make([]*spatialv1.VisibilityResult, 0, len(batchQuery.Queries))
	workBuffer := make([]uint32, 0, 100)

	for _, q := range batchQuery.Queries {
		visibleIDs := h.grid.GetInRadius(q.UserId, q.Radius, workBuffer)

		finalIDs := make([]uint32, len(visibleIDs))
		copy(finalIDs, visibleIDs)

		results = append(results, &spatialv1.VisibilityResult{
			UserId:         q.UserId,
			VisibleUserIds: finalIDs,
		})
	}

	response := &spatialv1.VisibilityBatchResponse{
		Results:   results,
		StateHash: h.grid.GetTotalHash(),
	}

	respBytes, err := proto.Marshal(response)
	if err != nil {
		return
	}

	msg.Respond(respBytes)
}
