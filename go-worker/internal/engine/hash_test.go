package engine

import (
	"testing"
)

func TestHashPlayer(t *testing.T) {
	// Identity check
	h1 := HashPlayer(1, 10.5, 20.5, 30.5)
	h2 := HashPlayer(1, 10.5, 20.5, 30.5)
	if h1 != h2 {
		t.Error("hashes should be identical for the same input")
	}

	// Change user ID
	h3 := HashPlayer(2, 10.5, 20.5, 30.5)
	if h1 == h3 {
		t.Error("hashes should be different for different user IDs")
	}

	// Change position (X)
	h4 := HashPlayer(1, 11.5, 20.5, 30.5)
	if h1 == h4 {
		t.Error("hashes should be different for different X")
	}
	
	// Change position (Y)
	h5 := HashPlayer(1, 10.5, 21.5, 30.5)
	if h1 == h5 {
		t.Error("hashes should be different for different Y")
	}
	
	// Change position (Z)
	h6 := HashPlayer(1, 10.5, 20.5, 31.5)
	if h1 == h6 {
		t.Error("hashes should be different for different Z")
	}
}

func TestHashPlayer_CommutativeXOR(t *testing.T) {
	// Simulate XORing multiple hashes
	h1 := HashPlayer(1, 10, 10, 10)
	h2 := HashPlayer(2, 20, 20, 20)
	h3 := HashPlayer(3, 30, 30, 30)

	sum1 := h1 ^ h2 ^ h3
	sum2 := h3 ^ h1 ^ h2
	sum3 := h2 ^ h3 ^ h1

	if sum1 != sum2 || sum2 != sum3 {
		t.Error("XOR sums should be commutative")
	}
}
