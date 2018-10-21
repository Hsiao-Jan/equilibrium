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

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
)

//go:generate gencodec -type txData -field-override txDataMarshaling -out gen_tx_json.go

type txData struct {
	AccountNonce uint64   `json:"accountNonce"    gencodec:"required"`
	ComputeLimit uint64   `json:"computeLimit"    gencodec:"required"`
	Receiver     *Address `json:"receiver"        rlp:"nil"` // nil means contract creation
	Amount       *big.Int `json:"amount"          gencodec:"required"`
	Payload      []byte   `json:"payload"         gencodec:"required"`

	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *Hash `json:"hash" rlp:"-"`
}

type txDataMarshaling struct {
	AccountNonce hexutil.Uint64
	ComputeLimit hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}

type Transaction struct {
	data txData

	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

func NewTransaction(nonce uint64, receiver Address, amount *big.Int, computeLimit uint64, payload []byte) *Transaction {
	return newTransaction(nonce, &receiver, amount, computeLimit, payload)
}

func NewContractCreation(nonce uint64, amount *big.Int, computeLimit uint64, payload []byte) *Transaction {
	return newTransaction(nonce, nil, amount, computeLimit, payload)
}

func newTransaction(nonce uint64, receiver *Address, amount *big.Int, computeLimit uint64, payload []byte) *Transaction {
	if len(payload) > 0 {
		data = common.CopyBytes(data)
	}

	data := txData{
		AccountNonce: nonce,
		Receiver:     receiver,
		Payload:      payload,
		Amount:       new(big.Int),
		ComputeLimit: computeLimit,
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
	}
	if amount != nil {
		data.Amount.Set(amount)
	}

	return &Transaction{data: data}
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(Hash)
	}
	v := rlpHash(tx)
	tx.hash.Store(v)
	return v
}

func (tx *Transaction) StabilityFee(stabilizationLevel uint64) *big.Int {
	return stability.CalcFee(tx.ComputeFee(), stabilizationLevel, tx.Value())
}

func (tx *Transaction) ComputeFee() *big.Int {
	return new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
}

// Cost returns the transaction cost for a specific stabilization level.
// NOTE: The result is used as an estimation as the transaction can be subject 
// to a different stabilization level as soon as it's committed.
func (tx *Transaction) Cost(stabilizationLevel uint64) *big.Int {
	total := new(big.Int).Add(tx.ComputeFee(), tx.StabilityFee(stabilizationLevel))
	return total.Add(total, tx.Value())
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// TxDifference returns a new set t which is the difference between a to b.
func TxDifference(a, b Transactions) (keep Transactions) {
	keep = make(Transactions, 0, len(a))

	remove := make(map[Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}
