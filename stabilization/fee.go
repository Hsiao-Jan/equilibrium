// Copyright © 2018 Kowala SEZC <info@kowala.tech>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stability

/*
var (
	Increase     = new(big.Int).Add(common.Big100, new(big.Int).SetUint64(params.StabilityIncreasePercentage))
	TxPercentage = new(big.Int).SetUint64(params.StabilityFeeTxPercentage)
)

// Fee returns the stability fee for a specific a compute fee, stabilization level and transaction amount.
func Fee(computeFee *big.Int, stabilizationLevel uint64, txAmount *big.Int) *big.Int {
	if stabilizationLevel == 0 {
		return common.Big0
	}

	if txAmount.Cmp(common.Big0) == 0 {
		return computeFee
	}

	// fee = compute fee  * 1.09^r(b)
	lvl := new(big.Int).SetUint64(stabilizationLevel)
	mul := new(big.Int).Exp(stabilityIncrease, lvl, nil)
	div := new(big.Int).Exp(common.Big100, lvl, nil)
	fee := new(big.Int).Div(new(big.Int).Mul(computeFee, mul), div)

	// percentage of tx amount
	maxFee := new(big.Int).Div(new(big.Int).Mul(txAmount, stabilityTxPercentage), common.Big100)

	return common.Min(fee, maxFee)
}
*/
