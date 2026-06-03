// Package engine implements the spatial partitioning and grid logic.
package engine

import (
	"context"
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
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewTelemetryHandler initializes a new telemetry handler with its own grid and object pool.
func NewTelemetryHandler(ctx context.Context) *TelemetryHandler {
	ctx, cancel := context.WithCancel(ctx)
	handler := &TelemetryHandler{
		pool: sync.Pool{
			New: func() any {
				return &spatialv1.TelemetryBatch{}
			},
		},
		grid:   NewSpatialGrid(),
		cancel: cancel,
	}

	handler.wg.Add(1)
	go handler.monitorRPS(ctx)

	return handler
}

func (h *TelemetryHandler) monitorRPS(ctx context.Context) {
	defer h.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			count := h.batchCount.Swap(0)
			if count > 0 {
				log.Printf("[Telemetry] Processing rate: %d batches/sec", count/10)
			}
		case <-ctx.Done():
			return
		}
	}
}

// Shutdown cancels the handler's internal context and waits for background
// goroutines to finish. Call this before stopping the NATS connection so
// no handler goroutines are left running.
func (h *TelemetryHandler) Shutdown() {
	h.cancel()
	h.wg.Wait()
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

// HandleFullState processes a full state snapshot from the node-plane during
// handshake. It bulk-replaces the grid with the complete set of players.
func (h *TelemetryHandler) HandleFullState(msg *nats.Msg) {
	batch := &spatialv1.TelemetryBatch{}
	if err := proto.Unmarshal(msg.Data, batch); err != nil {
		log.Printf("[Handshake] Failed to unmarshal full state: %v", err)
		return
	}

	h.grid.BulkUpdate(batch.Players)
	log.Printf("[Handshake] Full state applied: %d players (hash=%d)", len(batch.Players), h.grid.GetTotalHash())
}

// HandlePlayerRemove removes a player from the grid when the node-plane
// signals the player has left the world.
func (h *TelemetryHandler) HandlePlayerRemove(msg *nats.Msg) {
	remove := &spatialv1.PlayerRemove{}
	if err := proto.Unmarshal(msg.Data, remove); err != nil {
		return
	}

	h.grid.RemovePlayer(remove.UserId)
	log.Printf("[PlayerRemove] Player %d removed from grid (hash=%d)", remove.UserId, h.grid.GetTotalHash())
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

	// Take a snapshot of the current state hash BEFORE processing queries.
	// This ensures all query results in this batch are consistent with the
	// returned hash — no partial writes between queries can skew it.
	snapshotHash := h.grid.GetTotalHash()

	results := make([]*spatialv1.VisibilityResult, 0, len(batchQuery.Queries))
	workBuffer := make([]uint32, 0, 100)

	for _, q := range batchQuery.Queries {
		visibleIDs, _ := h.grid.GetInRadiusWithHash(q.UserId, q.Radius, workBuffer)

		finalIDs := make([]uint32, len(visibleIDs))
		copy(finalIDs, visibleIDs)

		results = append(results, &spatialv1.VisibilityResult{
			UserId:         q.UserId,
			VisibleUserIds: finalIDs,
		})
	}

	response := &spatialv1.VisibilityBatchResponse{
		Results:   results,
		StateHash: snapshotHash,
	}

	respBytes, err := proto.Marshal(response)
	if err != nil {
		return
	}

	if err := msg.Respond(respBytes); err != nil {
		log.Printf("[Telemetry] Error responding to query: %v", err)
	}
}
