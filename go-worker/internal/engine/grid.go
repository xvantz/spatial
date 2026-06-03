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

// cellEntry stores a player together with their position inline,
// avoiding an extra map lookup for position during proximity queries.
type cellEntry struct {
	UserID   uint32
	Position Position
}

// SpatialGrid implements a voxel-based spatial partitioning system.
// It uses a bucket-based approach for efficient proximity queries and
// maintains a commutative XOR hash of the entire world state for synchronization.
type SpatialGrid struct {
	mu           sync.RWMutex
	buckets      map[uint64][]cellEntry
	bucketIndex  map[uint32]int // userID -> index within its bucket slice, for O(1) swap-remove
	reverseIndex map[uint32]uint64
	positions    map[uint32]Position
	playerHashes map[uint32]uint64
	totalHash    uint64
}

// NewSpatialGrid initializes a new grid with pre-allocated maps for performance.
func NewSpatialGrid() *SpatialGrid {
	return &SpatialGrid{
		buckets:      make(map[uint64][]cellEntry, 10000),
		bucketIndex:  make(map[uint32]int, 1000),
		reverseIndex: make(map[uint32]uint64, 1000),
		positions:    make(map[uint32]Position, 1000),
		playerHashes: make(map[uint32]uint64, 1000),
	}
}

// GetCubeIndex calculates a unique 64-bit identifier for a 3D cell based on coordinates.
func (g *SpatialGrid) GetCubeIndex(x, y, z float32) uint64 {
	invCellSize := float32(1.0 / CellSize)
	//nolint:gosec // G115 is safe here as coordinates are within realistic bounds
	bx := uint16(int16(math.Floor(float64(x * invCellSize))))
	//nolint:gosec // G115 is safe here
	by := uint16(int16(math.Floor(float64(y * invCellSize))))
	//nolint:gosec // G115 is safe here
	bz := uint16(int16(math.Floor(float64(z * invCellSize))))

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

		g.buckets[newGridID] = append(g.buckets[newGridID], cellEntry{UserID: userID, Position: Position{X: x, Y: y, Z: z}})
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

	// Hoist center position to local float32 vars so the inner loop
	// avoids struct field dereferences.
	cx, cy, cz := centerPos.X, centerPos.Y, centerPos.Z
	invCellSize := float32(1.0 / CellSize)

	minBx := int16(math.Floor(float64((cx - radius) * invCellSize)))
	maxBx := int16(math.Floor(float64((cx + radius) * invCellSize)))
	minBy := int16(math.Floor(float64((cy - radius) * invCellSize)))
	maxBy := int16(math.Floor(float64((cy + radius) * invCellSize)))
	minBz := int16(math.Floor(float64((cz - radius) * invCellSize)))
	maxBz := int16(math.Floor(float64((cz + radius) * invCellSize)))

	radiusSq := radius * radius

	for bx := minBx; bx <= maxBx; bx++ {
		bxPart := uint64(uint16(bx)) << 32
		for by := minBy; by <= maxBy; by++ {
			//nolint:gosec // G115 is safe for spatial indexing
			byPart := bxPart | uint64(uint16(by))<<16
			for bz := minBz; bz <= maxBz; bz++ {
				//nolint:gosec // G115 is safe for spatial indexing
				gridID := byPart | uint64(uint16(bz))

				if entries, ok := g.buckets[gridID]; ok {
					for _, entry := range entries {
						if entry.UserID == userID {
							continue
						}

						// Read position directly from the bucket entry —
						// avoids an expensive map lookup for every distance check.
						dx := entry.Position.X - cx
						dy := entry.Position.Y - cy
						dz := entry.Position.Z - cz
						distSq := dx*dx + dy*dy + dz*dz

						if distSq <= radiusSq {
							buffer = append(buffer, entry.UserID)
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
		lastEntry := bucket[lastIdx]
		bucket[idx] = lastEntry
		g.bucketIndex[lastEntry.UserID] = idx
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
