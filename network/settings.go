package network

import (
	"math/big"
)

// @TODO (rgeraldes)
//go:generate

// Settings represents the network settings.
type Settings struct {
	ChainID          *big.Int `json:"chainID"          gencodec:"required"`
	ComputeCapacity  uint64   `json:"computeCapacity"  gencodec:"required"`
	ComputeUnitPrice *big.Int `json:"computeUnitPrice" gencodec:"required"`
}
