package transaction

import (
	"math/big"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/network/params"
)

var (
	increase        = new(big.Int).Add(common.Big100, new(big.Int).SetUint64(params.StabilityFeeIncreasePercentage))
	maxTxPercentage = new(big.Int).SetUint64(params.StabilityFeeMaxPercentage)
)

// StabilityFee returns the stability fee for a specific a compute fee, stabilization level and transaction amount.
func StabilityFee(computeFee *big.Int, stabilizationLevel uint64, txAmount *big.Int) *big.Int {
	if stabilizationLevel == 0 {
		return common.Big0
	}

	if txAmount.Cmp(common.Big0) == 0 {
		return computeFee
	}

	// fee = compute fee  * 1.09^r(b)
	lvl := new(big.Int).SetUint64(stabilizationLevel)
	mul := new(big.Int).Exp(increase, lvl, nil)
	div := new(big.Int).Exp(common.Big100, lvl, nil)
	currentFee := new(big.Int).Div(new(big.Int).Mul(computeFee, mul), div)
	maxFee := new(big.Int).Div(new(big.Int).Mul(txAmount, maxTxPercentage), common.Big100)

	return common.Min(currentFee, maxFee)
}

// ComputeFee returns the transaction's compute fee (max compute fee for contract calls).
func ComputeFee(tx *Transaction, computeUnitPrice *big.Int) *big.Int {
	return new(big.Int).Mul(computeUnitPrice, new(big.Int).SetUint64(tx.data.ComputeLimit))
}

// Cost returns the transaction cost for a specific stabilization level and cumpute unit price.
func Cost(tx *Transaction, stabilizationLevel uint64, computeUnitPrice *big.Int) *big.Int {
	computeFee := ComputeFee(tx, computeUnitPrice)
	return new(big.Int).Add(new(big.Int).Add(computeFee, StabilityFee(computeFee, stabilizationLevel, tx.Amount())), tx.Amount())
}
