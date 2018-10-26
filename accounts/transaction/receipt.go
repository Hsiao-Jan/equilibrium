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
	"bytes"
	"fmt"
	"io"
	"math/big"
	"unsafe"

	"github.com/kowala-tech/equilibrium/accounts"
	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
)

//go:generate gencodec -type Receipt -field-override receiptMarshaling -out gen_receipt_json.go

var (
	receiptStatusFailedRLP     = []byte{}
	receiptStatusSuccessfulRLP = []byte{0x01}
)

const (
	// ReceiptStatusFailed is the status code of a transaction if execution failed.
	ReceiptStatusFailed = uint64(0)

	// ReceiptStatusSuccessful is the status code of a transaction if execution succeeded.
	ReceiptStatusSuccessful = uint64(1)
)

// Receipt contains the results of a transaction.
type Receipt struct {
	PostState    []byte       `json:"postState"`
	Status       uint64       `json:"status"`
	Bloom        common.Bloom `json:"logsBloom"                     gencodec:"required"`
	Logs         []*Log       `json:"logs"                          gencodec:"required"`
	ComputeFee   *big.Int     `json:"computeFee"                    gencodec:"required"`
	StabilityFee *big.Int     `json:"stabilityFee"                  gencodec:"required"`

	// Derived fields
	TxHash          crypto.Hash      `json:"transactionHash"  gencodec:"required"`
	ContractAddress accounts.Address `json:"contractAddress"`
}

type receiptMarshaling struct {
	PostState    hexutil.Bytes
	Status       hexutil.Uint64
	ComputeFee   *hexutil.Big
	StabilityFee *hexutil.Big
}

// receiptRLP is the consensus encoding of a receipt.
type receiptRLP struct {
	PostStateOrStatus []byte
	Bloom             common.Bloom
	Logs              []*Log
	ComputeFee        *big.Int
	StabilityFee      *big.Int
}

type receiptStorageRLP struct {
	PostStateOrStatus []byte
	Bloom             common.Bloom
	TxHash            crypto.Hash
	ContractAddress   accounts.Address
	Logs              []*LogForStorage
	ComputeFee        *big.Int
	StabilityFee      *big.Int
}

// NewReceipt creates a barebone transaction receipt, copying the init fields.
func NewReceipt(root []byte, failed bool, computeFee, stabilityFee *big.Int) *Receipt {
	r := &Receipt{
		PostState:    common.CopyBytes(root),
		ComputeFee:   computeFee,
		StabilityFee: stabilityFee,
	}
	if failed {
		r.Status = ReceiptStatusFailed
	} else {
		r.Status = ReceiptStatusSuccessful
	}
	return r
}

// EncodeRLP implements rlp.Encoder, and flattens the consensus fields of a receipt
// into an RLP stream. If no post state is present, byzantium fork is assumed.
func (r *Receipt) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &receiptRLP{r.statusEncoding(), r.Bloom, r.Logs, r.ComputeFee, r.StabilityFee})
}

// DecodeRLP implements rlp.Decoder, and loads the consensus fields of a receipt
// from an RLP stream.
func (r *Receipt) DecodeRLP(s *rlp.Stream) error {
	var dec receiptRLP
	if err := s.Decode(&dec); err != nil {
		return err
	}
	if err := r.setStatus(dec.PostStateOrStatus); err != nil {
		return err
	}
	r.Bloom, r.Logs, r.ComputeFee, r.StabilityFee = dec.Bloom, dec.Logs, dec.ComputeFee, dec.StabilityFee
	return nil
}

func (r *Receipt) setStatus(postStateOrStatus []byte) error {
	switch {
	case bytes.Equal(postStateOrStatus, receiptStatusSuccessfulRLP):
		r.Status = ReceiptStatusSuccessful
	case bytes.Equal(postStateOrStatus, receiptStatusFailedRLP):
		r.Status = ReceiptStatusFailed
	case len(postStateOrStatus) == len(crypto.Hash{}):
		r.PostState = postStateOrStatus
	default:
		return fmt.Errorf("invalid receipt status %x", postStateOrStatus)
	}
	return nil
}

func (r *Receipt) statusEncoding() []byte {
	if len(r.PostState) == 0 {
		if r.Status == ReceiptStatusFailed {
			return receiptStatusFailedRLP
		}
		return receiptStatusSuccessfulRLP
	}
	return r.PostState
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (r *Receipt) Size() common.StorageSize {
	size := common.StorageSize(unsafe.Sizeof(*r)) + common.StorageSize(len(r.PostState))

	size += common.StorageSize(len(r.Logs)) * common.StorageSize(unsafe.Sizeof(Log{}))
	for _, log := range r.Logs {
		size += common.StorageSize(len(log.Topics)*crypto.HashLength + len(log.Data))
	}
	return size
}

// TotalAmount lists the total amount that has been payed.
func (r *Receipt) TotalAmount() *big.Int {
	return new(big.Int).Add(r.ComputeFee, r.StabilityFee)
}

func (r *Receipt) String() string {
	return fmt.Sprintf(`
TxHash:			 	%s
ContractAddress: 	%s
ComputeFee:         %#x
StabilityFee:       %#x
CumulativeGasUsed:	%d
Status: 			%d
Size: 				%s
Logs: %v
`, r.TxHash.String(), r.ContractAddress.String(), r.ComputeFee, r.StabilityFee, r.Status, r.Size().String(), r.Logs)
}

// ReceiptForStorage is a wrapper around a Receipt that flattens and parses the
// entire content of a receipt, as opposed to only the consensus fields originally.
type ReceiptForStorage Receipt

// EncodeRLP implements rlp.Encoder, and flattens all content fields of a receipt
// into an RLP stream.
func (r *ReceiptForStorage) EncodeRLP(w io.Writer) error {
	enc := &receiptStorageRLP{
		PostStateOrStatus: (*Receipt)(r).statusEncoding(),
		Bloom:             r.Bloom,
		TxHash:            r.TxHash,
		ContractAddress:   r.ContractAddress,
		Logs:              make([]*LogForStorage, len(r.Logs)),
		ComputeFee:        r.ComputeFee,
		StabilityFee:      r.StabilityFee,
	}
	for i, log := range r.Logs {
		enc.Logs[i] = (*LogForStorage)(log)
	}
	return rlp.Encode(w, enc)
}

// DecodeRLP implements rlp.Decoder, and loads both consensus and implementation
// fields of a receipt from an RLP stream.
func (r *ReceiptForStorage) DecodeRLP(s *rlp.Stream) error {
	var dec receiptStorageRLP
	if err := s.Decode(&dec); err != nil {
		return err
	}
	if err := (*Receipt)(r).setStatus(dec.PostStateOrStatus); err != nil {
		return err
	}

	r.Bloom, r.ComputeFee, r.StabilityFee = dec.Bloom, dec.ComputeFee, dec.StabilityFee
	r.Logs = make([]*Log, len(dec.Logs))
	for i, log := range dec.Logs {
		r.Logs[i] = (*Log)(log)
	}
	// Assign the implementation fields
	r.TxHash, r.ContractAddress = dec.TxHash, dec.ContractAddress
	return nil
}

// Receipts is a wrapper around a Receipt array to implement DerivableList.
type Receipts []*Receipt

// Len returns the number of receipts in this list.
func (r Receipts) Len() int { return len(r) }

// GetRlp returns the RLP encoding of one receipt from the list.
func (r Receipts) GetRlp(i int) []byte {
	bytes, err := rlp.EncodeToBytes(r[i])
	if err != nil {
		panic(err)
	}
	return bytes
}
