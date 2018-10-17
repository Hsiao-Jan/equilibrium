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
