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

package types

import (
	"math/big"

	"github.com/kowala-tech/kcoin/client/common"
)

// Receipt contains the results of a transaction.
type Receipt struct {
	// Consensus fields
	PostState                     []byte `json:"postState"`
	Status                        uint64 `json:"status"`
	CumulativeComputationalEffort uint64 `json:"cumulativeComputationalEffort" gencodec:"required"`
	Bloom                         Bloom  `json:"logsBloom"                     gencodec:"required"`
	Logs                          []*Log `json:"logs"                          gencodec:"required"`

	// Implementation fields (don't reorder!)
	TxHash          common.Hash    `json:"transactionHash"  gencodec:"required"`
	ContractAddress common.Address `json:"contractAddress"`
	ComputeFee      *big.Int       `json:"computeFee"       gencodec:"required"`
	StabilityFee    *big.Int       `json:"stabilityFee"     gencodec:"required"`
}
