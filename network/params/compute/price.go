package compute

import (
	"math/big"

	"github.com/kowala-tech/equilibrium/network"
)

var (
	// UnitCost maps a supportedd currency to its compute unit price.
	UnitCost = map[network.Currency]*big.Int{
		USD: new(big.Int).Mul(new(big.Int).SetUint64(400), new(big.Int).SetUint64(Shannon)),
	}
)
