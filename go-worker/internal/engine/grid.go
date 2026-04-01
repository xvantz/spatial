package engine

import (
	"math"
	"sync"
)

const CellSize = 50.0

type Position struct {
	X, Y, Z float32
}

type SpatialGrid struct {
	mu           sync.RWMutex
	buckets      map[uint64][]uint32
	reverseIndex map[uint32]uint64
	positions    map[uint32]Position
}

func NewSpatialGrid() *SpatialGrid {
	return &SpatialGrid{
		buckets:      make(map[uint64][]uint32, 10000),
		reverseIndex: make(map[uint32]uint64, 1000),
		positions:    make(map[uint32]Position, 1000),
	}
}

func (g *SpatialGrid) GetCubeIndex(x, y, z float32) uint64 {
	bx := int16(math.Floor(float64(x / CellSize)))
	by := int16(math.Floor(float64(y / CellSize)))
	bz := int16(math.Floor(float64(z / CellSize)))

	return uint64(uint16(bx))<<32 | uint64(uint16(by))<<16 | uint64(uint16(bz))
}

func (g *SpatialGrid) UpdatePosition(userID uint32, x, y, z float32) {
	newGridID := g.GetCubeIndex(x, y, z)

	g.mu.Lock()
	defer g.mu.Unlock()
	g.positions[userID] = Position{X: x, Y: y, Z: z}

	oldGridID, exists := g.reverseIndex[userID]
	if !exists || oldGridID != newGridID {
		if exists {
			g.removeFromBucket(userID, oldGridID)
		}

		g.buckets[newGridID] = append(g.buckets[newGridID], userID)
		g.reverseIndex[userID] = newGridID
	}
}

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

func (g *SpatialGrid) removeFromBucket(userID uint32, gridID uint64) {
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

func (g *SpatialGrid) RemovePlayer(userID uint32) {
	if gridID, exists := g.reverseIndex[userID]; exists {
		g.removeFromBucket(userID, gridID)
		delete(g.reverseIndex, userID)
	}
}
