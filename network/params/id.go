package params

import (
	"math/big"
)

var (
	// ChainID is a unique identifier for Kowala networks in order
	// to have replay attack protection.
	ChainID = new(big.Int).SetUint64(HDCoinType)
)
