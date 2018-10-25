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
	"fmt"
	"io"
	"math/big"
	"sort"
	"sync/atomic"
	"unsafe"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
)

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go
//go:generate gencodec -type Commit -out gen_commit_json.go

var (
	// EmptyRootHash represents a trie hash for an empty slice of transactions.
	EmptyRootHash = deriveSha(Transactions{})
	// EmptyHash represents the rlp hash for nil.
	EmptyHash = rlpHash(nil)
)

// Header represents a block header.
type Header struct {
	// basic info
	Number            *big.Int `json:"number"            gencodec:"required"`
	PreviousBlockHash Hash     `json:"previousBlockHash" gencodec:"required"`
	Extra             []byte   `json:"extraData"         gencodec:"required"`

	// consensus
	Snapshot Hash     `json:"stateRoot" gencodec:"required"`
	Time     *big.Int `json:"timestamp" gencodec:"required"` // time is used to sync the validators upon a new consensus round.
	Proposer Address  `json:"proposer"  gencodec:"required"`

	// block data
	LastCommitHash        Hash  `json:"lastCommitRoot"  gencodec:"required"`
	ProtocolViolationHash Hash  `json:"violationsHash"  gencodec:"required"`
	TxHash                Hash  `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash           Hash  `json:"receiptHash"      gencodec:"required"`
	Bloom                 Bloom `json:"logsBloom"        gencodec:"required"`
}

// headerMarshaling field type overrides for gencodec
type headerMarshaling struct {
	Number *hexutil.Big
	Extra  hexutil.Bytes
	Time   *hexutil.Big
	Hash   Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() Hash {
	return rlpHash(h)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() StorageSize {
	return StorageSize(unsafe.Sizeof(*h)) + StorageSize(len(h.Extra)+(h.Number.BitLen()+h.Time.BitLen())/8)
}

// CopyHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Time = new(big.Int); h.Time != nil {
		cpy.Time.Set(h.Time)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	return &cpy
}

// Block represents the network unit.
type Block struct {
	header             *Header
	lastCommit         *Commit
	protocolViolations []*Conviction
	transactions       Transactions

	// caches
	hash atomic.Value
	size atomic.Value
}

// NewBlock creates a new block. The values of TxHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs and receipts.
func NewBlock(header *Header, txs []*Transaction, receipts []*Receipt, lastCommit *Commit, violations []*Conviction) (*Block, error) {
	if len(txs) != len(receipts) {
		return nil, fmt.Errorf("Number of transactions (%d) does not match number of receipts (%d)", len(txs), len(receipts))
	}

	block := &Block{header: CopyHeader(header)}

	if len(txs) == 0 {
		block.header.TxHash = EmptyRootHash
	} else {
		block.header.TxHash = deriveSha(Transactions(txs))
		block.transactions = make(Transactions, len(txs))
		copy(block.transactions, txs)
	}

	if len(receipts) == 0 {
		block.header.ReceiptHash = EmptyRootHash
	} else {
		block.header.ReceiptHash = deriveSha(Receipts(receipts))
		block.header.Bloom = CreateBloom(receipts)
	}

	if lastCommit != nil {
		block.header.LastCommitHash = rlpHash(lastCommit)
		block.lastCommit = CopyCommit(lastCommit)
	}

	if len(violations) == 0 {
		block.header.ProtocolViolationHash = EmptyHash
	} else {
		block.header.ProtocolViolationHash = rlpHash(violations)
		block.protocolViolations = make([]*Conviction, len(violations))
		copy(block.protocolViolations, violations)
	}

	return block, nil
}

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *Block) Size() StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(StorageSize(c))
	return StorageSize(c)
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents together.
type Body struct {
	Transactions       []*Transaction
	LastCommit         *Commit
	ProtocolViolations []*Conviction
}

// Body returns the non-header content of the block.
func (b *Block) Body() *Body {
	return &Body{
		Transactions:       b.transactions,
		LastCommit:         b.lastCommit,
		ProtocolViolations: b.protocolViolations,
	}
}

// WithBody returns a new block with the given transaction, commit and protocol violations contents.
func (b *Block) WithBody(txs []*Transaction, lastCommit *Commit, protocolViolations []*Conviction) *Block {
	block := &Block{
		header:             CopyHeader(b.header),
		transactions:       make([]*Transaction, len(txs)),
		protocolViolations: make([]*Conviction, len(protocolViolations)),
	}
	copy(block.transactions, txs)
	copy(block.protocolViolations, protocolViolations)

	if lastCommit != nil {
		block.lastCommit = CopyCommit(lastCommit)
	}

	return block
}

// Header returns a deep copy of the block header.
func (b *Block) Header() *Header { return CopyHeader(b.header) }

// Transactions returns the block's transactions.
func (b *Block) Transactions() Transactions { return b.transactions }

// LastCommit returns the list of pre-commits for the previous block.
func (b *Block) LastCommit() *Commit { return CopyCommit(b.lastCommit) }

// ProtocolViolations returns the list of convictions.
func (b *Block) ProtocolViolations() []*Conviction { return b.protocolViolations }

// Transaction returns a transaction for a given hash if the transaction
// is present in the block.
func (b *Block) Transaction(hash Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}

// Number returns the block number.
func (b *Block) Number() *big.Int { return new(big.Int).Set(b.header.Number) }

// NumberU64 returns the block number as uint64.
func (b *Block) NumberU64() uint64 { return b.header.Number.Uint64() }

// PreviousBlockHash returns the block hash of the previous chain block.
func (b *Block) PreviousBlockHash() Hash { return b.header.PreviousBlockHash }

// Extra returns extra information present in the block.
func (b *Block) Extra() []byte { return common.CopyBytes(b.header.Extra) }

// Time returns
func (b *Block) Time() *big.Int { return new(big.Int).Set(b.header.Time) }

// Proposer returns the validator responsible for proposing the block.
func (b *Block) Proposer() Address { return b.header.Proposer }

// LastCommitHash returns the hash of the +2/3 precommit signatures for the previous block.
func (b *Block) LastCommitHash() Hash { return b.header.LastCommitHash }

// ProtocolViolationHash returns the hash of the protocol violations.
func (b *Block) ProtocolViolationHash() Hash { return b.header.ProtocolViolationHash }

// Snapshot returns the block's state root.
func (b *Block) Snapshot() Hash { return b.header.Snapshot }

// Bloom returns the logs bloom filter.
func (b *Block) Bloom() Bloom { return b.header.Bloom }

// TxHash returns the transactions' trie root.
func (b *Block) TxHash() Hash { return b.header.TxHash }

// ReceiptHash returns the receipts' trie root.
func (b *Block) ReceiptHash() Hash { return b.header.ReceiptHash }

// "external" block encoding.
type extblock struct {
	Header             *Header
	Transactions       []*Transaction
	LastCommit         *Commit
	ProtocolViolations []*Conviction
}

// DecodeRLP decodes the block.
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.transactions, b.lastCommit, b.protocolViolations = eb.Header, eb.Transactions, eb.LastCommit, eb.ProtocolViolations
	b.size.Store(StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header:             b.header,
		Transactions:       b.transactions,
		LastCommit:         b.lastCommit,
		ProtocolViolations: b.protocolViolations,
	})
}

// Blocks is a block slice type for sorting.
type Blocks []*Block

type BlockBy func(b1, b2 *Block) bool

func (bb BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     bb,
	}
	sort.Sort(bs)
}

type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 *Block) bool
}

// Len returns the length of s.
func (bs blockSorter) Len() int { return len(bs.blocks) }

// Swap swaps the i'th and the j'th element in bs.
func (bs blockSorter) Swap(i, j int) {
	bs.blocks[i], bs.blocks[j] = bs.blocks[j], bs.blocks[i]
}

// Less verifies if the i'th block comes before the j'th block.
func (bs blockSorter) Less(i, j int) bool { return bs.by(bs.blocks[i], bs.blocks[j]) }

// Commit contains the evidence that the block was committed by a set of validators.
type Commit struct {
	preCommits Votes `json:"preCommits" gencodec:"required"`
}

// PreCommits returns the validators' pre commits.
func (c *Commit) PreCommits() Votes { return c.preCommits }

// CopyCommit creates a deep copy of the commit info to prevent side effects from
// modifying a header variable.
func CopyCommit(commit *Commit) *Commit {
	cpy := *commit
	cpy.preCommits = make(Votes, len(commit.preCommits))
	copy(cpy.preCommits, commit.preCommits)
	return &cpy
}
