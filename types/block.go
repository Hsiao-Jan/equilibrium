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
