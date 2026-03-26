package engine

import "math"

const CellSize = 50.0

type SpatialGrid struct {
	buckets      map[uint64][]uint32
	reverseIndex map[uint32]uint64
}

func NewSpatialGrid() *SpatialGrid {
	return &SpatialGrid{
		buckets:      make(map[uint64][]uint32, 10000),
		reverseIndex: make(map[uint32]uint64, 1000),
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
	oldGridID, exists := g.reverseIndex[userID]

	if !exists || oldGridID != newGridID {
		if exists {
			g.removeFromBucket(userID, oldGridID)
		}

		g.buckets[newGridID] = append(g.buckets[newGridID], userID)
		g.reverseIndex[userID] = newGridID
	}
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
