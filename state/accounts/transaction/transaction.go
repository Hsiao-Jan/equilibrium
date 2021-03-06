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

package transaction

import (
	"io"
	"math/big"
	"sync/atomic"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/crypto/signer"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
	"github.com/kowala-tech/equilibrium/state/accounts"
)

//go:generate gencodec -type txData -field-override txDataMarshaling -out gen_transaction_json.go

type txData struct {
	AccountNonce uint64            `json:"accountNonce" gencodec:"required"`
	ComputeLimit uint64            `json:"computeLimit" gencodec:"required"`
	Recipient    *accounts.Address `json:"recipient"           rlp:"nil"` // nil means contract creation
	Amount       *big.Int          `json:"amount"       gencodec:"required"`
	Payload      []byte            `json:"payload"      gencodec:"required"`

	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *crypto.Hash `json:"hash" rlp:"-"`
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

// Transaction refers to the signed that package that stores a message to be
// sent from an externally owned account to another account on the blockchain.
type Transaction struct {
	data txData

	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

func NewTransaction(nonce uint64, recipient accounts.Address, amount *big.Int, computeLimit uint64, payload []byte) *Transaction {
	return newTransaction(nonce, &recipient, amount, computeLimit, payload)
}

func NewContractCreation(nonce uint64, amount *big.Int, computeLimit uint64, payload []byte) *Transaction {
	return newTransaction(nonce, nil, amount, computeLimit, payload)
}

func newTransaction(nonce uint64, recipient *accounts.Address, amount *big.Int, computeLimit uint64, payload []byte) *Transaction {
	if len(payload) > 0 {
		payload = common.CopyBytes(payload)
	}

	data := txData{
		AccountNonce: nonce,
		Recipient:    recipient,
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

// Payload returns the transaction data payload.
func (tx *Transaction) Payload() []byte { return common.CopyBytes(tx.data.Payload) }

// ComputeLimit returns the maximum required computational effort for the transaction execution.
func (tx *Transaction) ComputeLimit() uint64 { return tx.data.ComputeLimit }

// Amount returns the transaction amount.
func (tx *Transaction) Amount() *big.Int { return new(big.Int).Set(tx.data.Amount) }

// Nonce represents the account nonce.
func (tx *Transaction) Nonce() uint64 { return tx.data.AccountNonce }

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *Transaction) To() *accounts.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

// SignatureValues returns the transaction's raw signature values.
func (tx *Transaction) SignatureValues() (R, S, V *big.Int) {
	R, S, V = tx.data.R, tx.data.S, tx.data.V
	return
}

// Hash hashes the RLP encoding of tx. It uniquely identifies the transaction.
func (tx *Transaction) Hash() crypto.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(crypto.Hash)
	}
	v := crypto.RLPHash(tx)
	tx.hash.Store(v)
	return v
}

// HashWithData returns the transaction hash to be signed by the sender.
func (tx *Transaction) HashWithData(data ...interface{}) crypto.Hash {
	txData := []interface{}{
		tx.data.AccountNonce,
		tx.data.ComputeLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
	}
	return crypto.RLPHash(append(txData, data...))
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := common.WriteCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// Protected specifies whether the transaction is protected from replay attacks or not.
func (tx *Transaction) Protected() bool { return isProtectedV(tx.data.V) }

// ChainID derives the transaction chain ID from the signature.
func (tx *Transaction) ChainID() *big.Int {
	return common.DeriveChainID(tx.data.V)
}

// WithSignature returns a new transaction with the given signature.
func (tx *Transaction) WithSignature(signer signer.Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// EncodeRLP satisfies rlp.Encoder.
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP satisfies rlp.Decoder.
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// MarshalJSON encodes the RPC transaction format.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var data txData
	if err := data.UnmarshalJSON(input); err != nil {
		return err
	}

	withSignature := data.V.Sign() != 0 || data.R.Sign() != 0 || data.S.Sign() != 0
	if withSignature {
		var V byte
		if isProtectedV(data.V) {
			chainID := common.DeriveChainID(data.V).Uint64()
			V = byte(data.V.Uint64() - 35 - 2*chainID)
		} else {
			V = byte(data.V.Uint64() - 27)
		}
		if !crypto.ValidateSignatureValues(V, data.R, data.S, false) {
			return crypto.ErrInvalidSig
		}
	}

	*tx = Transaction{data: data}
	return nil
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

// TxDifference returns a new set of transactions consisting in the difference between a and b.
func TxDifference(a, b Transactions) (keep Transactions) {
	keep = make(Transactions, 0, len(a))

	remove := make(map[crypto.Hash]struct{})
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

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 are considered unprotected
	return true
}
