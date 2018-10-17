package types

import (
	"math/big"
	"sync/atomic"

	"github.com/kowala-tech/kcoin/client/common"
	"github.com/kowala-tech/kcoin/client/common/hexutil"
)

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// Header represents a block header in the Kowala blockchain.
type Header struct {
	ParentHash          Hash     `json:"parentHash"          gencodec:"required"`
	Number              *big.Int `json:"number"              gencodec:"required"`
	Coinbase            Address  `json:"miner"               gencodec:"required"`
	Root                Hash     `json:"stateRoot"           gencodec:"required"`
	TxHash              Hash     `json:"transactionsRoot"    gencodec:"required"`
	ReceiptHash         Hash     `json:"receiptHash"         gencodec:"required"`
	Bloom               Bloom    `json:"logsBloom"           gencodec:"required"`
	ComputationalEffort uint64   `json:"computationalEffort" gencodec:"required"`
	Time                *big.Int `json:"timestamp"           gencodec:"required"`
	Extra               []byte   `json:"extraData"           gencodec:"required"`
}

// headerMarshaling field type overrides for gencodec
type headerMarshaling struct {
	Number              *hexutil.Big
	ComputationalEffort hexutil.Uint64
	Time                *hexutil.Big
	Extra               hexutil.Byte
	Hash                common.Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

type Block struct {
	header *Header
	txs    Transactions

	// caches
	hash atomic.Value
	size atomic.Value
}
