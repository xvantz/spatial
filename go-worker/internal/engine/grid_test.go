package engine

import (
	"testing"
)

func TestSpatialGrid_UpdateAndHash(t *testing.T) {
	grid := NewSpatialGrid()

	// Initial hash should be 0
	if grid.GetTotalHash() != 0 {
		t.Errorf("expected initial hash 0, got %v", grid.GetTotalHash())
	}

	// Update player 1
	grid.UpdatePosition(1, 10.0, 10.0, 10.0)
	h1 := grid.GetTotalHash()
	if h1 == 0 {
		t.Error("hash should not be 0 after update")
	}

	// Move player 1
	grid.UpdatePosition(1, 20.0, 20.0, 20.0)
	h2 := grid.GetTotalHash()
	if h1 == h2 {
		t.Error("hash should change after player move")
	}

	// Move player 1 back
	grid.UpdatePosition(1, 10.0, 10.0, 10.0)
	if grid.GetTotalHash() != h1 {
		t.Error("hash should be identical when returning to previous state")
	}

	// Remove player 1
	grid.RemovePlayer(1)
	if grid.GetTotalHash() != 0 {
		t.Errorf("expected hash 0 after player removal, got %v", grid.GetTotalHash())
	}
}

func TestSpatialGrid_GetInRadius(t *testing.T) {
	grid := NewSpatialGrid()

	// Setup: P1 at (0,0,0), P2 at (10,0,0), P3 at (100,100,100)
	grid.UpdatePosition(1, 0, 0, 0)
	grid.UpdatePosition(2, 10, 0, 0)
	grid.UpdatePosition(3, 100, 100, 100)

	// Query from P1 with radius 15 (should see P2)
	buffer := make([]uint32, 0, 10)
	visible := grid.GetInRadius(1, 15, buffer)

	if len(visible) != 1 {
		t.Fatalf("expected 1 visible player, got %d", len(visible))
	}
	if visible[0] != 2 {
		t.Errorf("expected player 2, got %d", visible[0])
	}

	// Query from P1 with radius 5 (should see no one)
	visible = grid.GetInRadius(1, 5, buffer)
	if len(visible) != 0 {
		t.Errorf("expected 0 visible players, got %d", len(visible))
	}

	// Query from P1 with radius 200 (should see P2 and P3)
	visible = grid.GetInRadius(1, 200, buffer)
	if len(visible) != 2 {
		t.Errorf("expected 2 visible players, got %d", len(visible))
	}
}

func TestSpatialGrid_BulkUpdate(t *testing.T) {
	grid := NewSpatialGrid()

	// Initial players
	grid.UpdatePosition(1, 0, 0, 0)
	grid.UpdatePosition(2, 50, 50, 50)

	hashBefore := grid.GetTotalHash()

	// Move them both
	grid.UpdatePosition(1, 1, 1, 1)
	grid.UpdatePosition(2, 51, 51, 51)

	hashAfter := grid.GetTotalHash()
	if hashBefore == hashAfter {
		t.Error("hash should change after bulk move simulation")
	}
}

func TestSpatialGrid_CommutativeHash(t *testing.T) {
	g1 := NewSpatialGrid()
	g2 := NewSpatialGrid()

	// G1: Add P1 then P2
	g1.UpdatePosition(1, 10, 10, 10)
	g1.UpdatePosition(2, 20, 20, 20)

	// G2: Add P2 then P1
	g2.UpdatePosition(2, 20, 20, 20)
	g2.UpdatePosition(1, 10, 10, 10)

	if g1.GetTotalHash() != g2.GetTotalHash() {
		t.Errorf("XOR hash must be commutative: %v != %v", g1.GetTotalHash(), g2.GetTotalHash())
	}
}

func BenchmarkSpatialGrid_GetInRadius(b *testing.B) {
	grid := NewSpatialGrid()
	numPlayers := 1000

	for i := 0; i < numPlayers; i++ {
		//nolint:gosec // G115 is safe for benchmark indices
		grid.UpdatePosition(uint32(i), float32(i), float32(i), float32(i))
	}

	buffer := make([]uint32, 0, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = grid.GetInRadius(0, 100.0, buffer)
	}
}
