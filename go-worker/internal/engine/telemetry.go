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
		grid: NewSpatialGrid(),
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

	h.grid.BulkUpdate(batch.Players)

	if batch.StateHash != 0 && batch.StateHash != h.grid.GetTotalHash() {
		log.Printf(
			"[DESYNC] Received Hash: %v | Current Hash: %v",
			batch.StateHash,
			h.grid.GetTotalHash(),
		)
	}

	h.batchCount.Add(1)
}

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
