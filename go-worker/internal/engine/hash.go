package engine

import (
	"encoding/binary"
	"math"
)

const (
	offset64 = 14695981039346656037
	prime64  = 1099511628211
)

// HashPlayer calculates a commutative XOR hash for a player's ID and coordinates.
func HashPlayer(userID uint32, x, y, z float32) uint64 {
	h := uint64(offset64)

	// UserID
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], userID)
	for _, b := range buf {
		h ^= uint64(b)
		h *= prime64
	}

	// X
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(x))
	for _, b := range buf {
		h ^= uint64(b)
		h *= prime64
	}

	// Y
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(y))
	for _, b := range buf {
		h ^= uint64(b)
		h *= prime64
	}

	// Z
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(z))
	for _, b := range buf {
		h ^= uint64(b)
		h *= prime64
	}

	return h
}
