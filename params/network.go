package params

import "math/big"

// These are network parameters that need to be constant between
// clients, but aren't necesarilly consensus related.

var (
	// ComputeUnitPrice represents the network's compute unit price (400 Gwei/Shannon).
	ComputeUnitPrice = new(big.Int).Mul(new(big.Int).SetUint64(400), new(big.Int).SetUint64(Shannon))
)
