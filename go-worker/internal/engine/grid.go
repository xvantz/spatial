package engine

import (
	"math"
	spatialv1 "spatial/gen/spatial/v1"
	"sync"
)

const CellSize = 50.0

// Position represents 3D coordinates in the spatial world.
type Position struct {
	X, Y, Z float32
}

// SpatialGrid implements a voxel-based spatial partitioning system.
// It uses a bucket-based approach for efficient proximity queries and
// maintains a commutative XOR hash of the entire world state for synchronization.
type SpatialGrid struct {
	mu           sync.RWMutex
	buckets      map[uint64][]uint32
	reverseIndex map[uint32]uint64
	positions    map[uint32]Position
	playerHashes map[uint32]uint64
	totalHash    uint64
}

// NewSpatialGrid initializes a new grid with pre-allocated maps for performance.
func NewSpatialGrid() *SpatialGrid {
	return &SpatialGrid{
		buckets:      make(map[uint64][]uint32, 10000),
		reverseIndex: make(map[uint32]uint64, 1000),
		positions:    make(map[uint32]Position, 1000),
		playerHashes: make(map[uint32]uint64, 1000),
	}
}

// GetCubeIndex calculates a unique 64-bit identifier for a 3D cell based on coordinates.
func (g *SpatialGrid) GetCubeIndex(x, y, z float32) uint64 {
	bx := int16(math.Floor(float64(x / CellSize)))
	by := int16(math.Floor(float64(y / CellSize)))
	bz := int16(math.Floor(float64(z / CellSize)))

	return uint64(uint16(bx))<<32 | uint64(uint16(by))<<16 | uint64(uint16(bz))
}

// UpdatePosition updates a single player's position and recalculates the global state hash.
func (g *SpatialGrid) UpdatePosition(userID uint32, x, y, z float32) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.updatePositionLocked(userID, x, y, z)
}

// BulkUpdate processes multiple player updates atomically under a single write lock.
// This prevents desynchronization reports caused by reading partial world states.
func (g *SpatialGrid) BulkUpdate(players []*spatialv1.PlayerDelta) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, p := range players {
		g.updatePositionLocked(p.UserId, p.Position.X, p.Position.Y, p.Position.Z)
	}
}

// updatePositionLocked performs the actual position update and bucket management.
// Must be called with the write lock held.
func (g *SpatialGrid) updatePositionLocked(userID uint32, x, y, z float32) {
	newGridID := g.GetCubeIndex(x, y, z)
	newHash := HashPlayer(userID, x, y, z)

	if oldHash, exists := g.playerHashes[userID]; exists {
		g.totalHash ^= oldHash
	}
	g.playerHashes[userID] = newHash
	g.totalHash ^= newHash

	g.positions[userID] = Position{X: x, Y: y, Z: z}

	oldGridID, exists := g.reverseIndex[userID]
	if !exists || oldGridID != newGridID {
		if exists {
			g.removeFromBucketLocked(userID, oldGridID)
		}

		g.buckets[newGridID] = append(g.buckets[newGridID], userID)
		g.reverseIndex[userID] = newGridID
	}
}

// GetInRadius performs a spherical proximity query, returning user IDs within the specified radius.
func (g *SpatialGrid) GetInRadius(userID uint32, radius float32, buffer []uint32) []uint32 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	buffer = buffer[:0]

	centerPos, exists := g.positions[userID]
	if !exists {
		return buffer
	}

	minBx := int16(math.Floor(float64((centerPos.X - radius) / CellSize)))
	maxBx := int16(math.Floor(float64((centerPos.X + radius) / CellSize)))

	minBy := int16(math.Floor(float64((centerPos.Y - radius) / CellSize)))
	maxBy := int16(math.Floor(float64((centerPos.Y + radius) / CellSize)))

	minBz := int16(math.Floor(float64((centerPos.Z - radius) / CellSize)))
	maxBz := int16(math.Floor(float64((centerPos.Z + radius) / CellSize)))

	radiusSq := radius * radius

	for bx := minBx; bx <= maxBx; bx++ {
		for by := minBy; by <= maxBy; by++ {
			for bz := minBz; bz <= maxBz; bz++ {
				gridID := uint64(uint16(bx))<<32 | uint64(uint16(by))<<16 | uint64(uint16(bz))

				if players, ok := g.buckets[gridID]; ok {
					for _, targetID := range players {
						if targetID == userID {
							continue
						}

						targetPos := g.positions[targetID]

						dx := targetPos.X - centerPos.X
						dy := targetPos.Y - centerPos.Y
						dz := targetPos.Z - centerPos.Z
						distSq := dx*dx + dy*dy + dz*dz

						if distSq <= radiusSq {
							buffer = append(buffer, targetID)
						}
					}
				}
			}
		}
	}

	return buffer
}

func (g *SpatialGrid) removeFromBucketLocked(userID uint32, gridID uint64) {
	bucket := g.buckets[gridID]
	lastIdx := len(bucket) - 1

	for i, id := range bucket {
		if id == userID {
			bucket[i] = bucket[lastIdx]
			g.buckets[gridID] = bucket[:lastIdx]
			return
		}
	}
}

// GetTotalHash returns the current commutative XOR hash of the entire grid.
func (g *SpatialGrid) GetTotalHash() uint64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.totalHash
}

// RemovePlayer cleans up a player's data and updates the global state hash.
func (g *SpatialGrid) RemovePlayer(userID uint32) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if hash, exists := g.playerHashes[userID]; exists {
		g.totalHash ^= hash
		delete(g.playerHashes, userID)
	}

	if gridID, exists := g.reverseIndex[userID]; exists {
		g.removeFromBucketLocked(userID, gridID)
		delete(g.reverseIndex, userID)
		delete(g.positions, userID)
	}
}
