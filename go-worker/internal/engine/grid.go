// Package engine implements the spatial partitioning and grid logic.
package engine

import (
	"math"
	spatialv1 "spatial/gen/spatial/v1"
	"sync"
)

// CellSize defines the size of a single grid cell (voxel) in meters.
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
	bucketIndex  map[uint32]int // userID -> index within its bucket slice, for O(1) swap-remove
	reverseIndex map[uint32]uint64
	positions    map[uint32]Position
	playerHashes map[uint32]uint64
	totalHash    uint64
}

// NewSpatialGrid initializes a new grid with pre-allocated maps for performance.
func NewSpatialGrid() *SpatialGrid {
	return &SpatialGrid{
		buckets:      make(map[uint64][]uint32, 10000),
		bucketIndex:  make(map[uint32]int, 1000),
		reverseIndex: make(map[uint32]uint64, 1000),
		positions:    make(map[uint32]Position, 1000),
		playerHashes: make(map[uint32]uint64, 1000),
	}
}

// GetCubeIndex calculates a unique 64-bit identifier for a 3D cell based on coordinates.
func (g *SpatialGrid) GetCubeIndex(x, y, z float32) uint64 {
	//nolint:gosec // G115 is safe here as coordinates are within realistic bounds
	bx := uint16(int16(math.Floor(float64(x / CellSize))))
	//nolint:gosec // G115 is safe here
	by := uint16(int16(math.Floor(float64(y / CellSize))))
	//nolint:gosec // G115 is safe here
	bz := uint16(int16(math.Floor(float64(z / CellSize))))

	return uint64(bx)<<32 | uint64(by)<<16 | uint64(bz)
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
		g.bucketIndex[userID] = len(g.buckets[newGridID]) - 1
		g.reverseIndex[userID] = newGridID
	}
}

// GetInRadiusWithHash performs a spherical proximity query, returning user IDs
// within the specified radius AND the current world state hash atomically
// under a single read lock. This ensures the returned hash is consistent with
// the query results.
func (g *SpatialGrid) GetInRadiusWithHash(userID uint32, radius float32, buffer []uint32) ([]uint32, uint64) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	buffer = buffer[:0]

	centerPos, exists := g.positions[userID]
	if !exists {
		return buffer, g.totalHash
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
				//nolint:gosec // G115 is safe for spatial indexing
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

	return buffer, g.totalHash
}

// GetInRadius performs a spherical proximity query, returning user IDs within the specified radius.
func (g *SpatialGrid) GetInRadius(userID uint32, radius float32, buffer []uint32) []uint32 {
	visible, _ := g.GetInRadiusWithHash(userID, radius, buffer)
	return visible
}

// removeFromBucketLocked removes a user from a bucket in O(1) time by swapping
// with the last element and shrinking the slice. Must be called with the write lock held.
func (g *SpatialGrid) removeFromBucketLocked(userID uint32, gridID uint64) {
	bucket := g.buckets[gridID]
	idx, ok := g.bucketIndex[userID]
	if !ok {
		return
	}

	lastIdx := len(bucket) - 1
	if idx != lastIdx {
		lastID := bucket[lastIdx]
		bucket[idx] = lastID
		g.bucketIndex[lastID] = idx
	}

	g.buckets[gridID] = bucket[:lastIdx]
	delete(g.bucketIndex, userID)

	if lastIdx == 0 {
		delete(g.buckets, gridID)
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
