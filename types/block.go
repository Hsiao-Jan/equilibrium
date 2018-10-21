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
	"reflect"
	"sort"
	"sync/atomic"
	"unsafe"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
)

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

var (
	EmptyRootHash = deriveSha(Transactions{})
	EmptyHash     = rlpHash(nil)
)

// Header represents a block header in the Kowala blockchain.
type Header struct {
	// basic info
	Number     *big.Int `json:"number"    gencodec:"required"`
	ParentHash Hash     `json:"parentHash"     gencodec:"required"`
	Extra      []byte   `json:"extraData" gencodec:"required"`

	// consensus
	Time                  *big.Int `json:"timestamp"      gencodec:"required"`
	Proposer              Address  `json:"proposer"       gencodec:"required"`
	LastCommitHash        Hash     `json:"lastCommitRoot" gencodec:"required"`
	ProtocolViolationHash Hash     `json:"violationsHash" gencodec:"required"`
	Root                  Hash     `json:"stateRoot"      gencodec:"required"`

	// block data
	TxHash      Hash  `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash Hash  `json:"receiptHash"      gencodec:"required"`
	Bloom       Bloom `json:"logsBloom"        gencodec:"required"`
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
	protocolViolations Convictions
	transactions       Transactions

	// caches
	hash atomic.Value
	size atomic.Value
}

// NewBlock creates a new block. The values of TxHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs and receipts.
func NewBlock(header *Header, txs []*Transaction, receipts []*Receipt, lastCommit *Commit, violations Convictions) (*Block, error) {
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
		// @TODO
		//block.header.LastCommitHash = deriveSha()
		block.lastCommit = CopyCommit(lastCommit)
	}

	if len(violations) == 0 {
		block.header.ProtocolViolationHash = EmptyHash
	} else {
		block.header.ProtocolViolationHash = deriveSha(Convictions(violations))
		block.protocolViolations = make(Convictions, len(violations))
		copy(block.protocolViolations, violations)
	}

	return block, nil
}

type writeCounter StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
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

func (b *Block) Header() *Header                 { return CopyHeader(b.header) }
func (b *Block) Transactions() Transactions      { return b.transactions }
func (b *Block) LastCommit() *Commit             { return CopyCommit(b.lastCommit) }
func (b *Block) ProtocolViolations() Convictions { return b.protocolViolations }

func (b *Block) Transaction(hash Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}

func (b *Block) Number() *big.Int  { return new(big.Int).Set(b.header.Number) }
func (b *Block) NumberU64() uint64 { return b.header.Number.Uint64() }
func (b *Block) ParentHash() Hash  { return b.header.ParentHash }
func (b *Block) Extra() []byte     { return common.CopyBytes(b.header.Extra) }

func (b *Block) Time() *big.Int              { return new(big.Int).Set(b.header.Time) }
func (b *Block) Proposer() Address           { return b.header.Proposer }
func (b *Block) LastCommitHash() Hash        { return b.header.LastCommitHash }
func (b *Block) ProtocolViolationHash() Hash { return b.header.ProtocolViolationHash }
func (b *Block) Root() Hash                  { return b.header.Root }

func (b *Block) Bloom() Bloom      { return b.header.Bloom }
func (b *Block) TxHash() Hash      { return b.header.TxHash }
func (b *Block) ReceiptHash() Hash { return b.header.ReceiptHash }

// "external" block encoding. used for kcoin protocol, etc.
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

type Blocks []*Block

type BlockBy func(b1, b2 *Block) bool

func (self BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     self,
	}
	sort.Sort(bs)
}

type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 *Block) bool
}

func (self blockSorter) Len() int { return len(self.blocks) }
func (self blockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self blockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }

func Number(b1, b2 *Block) bool { return b1.header.Number.Cmp(b2.header.Number) < 0 }

// Commit contains the evidence that the block was committed by a set of validators.
type Commit struct {
	preCommits Votes `json:"preCommits" gencodec:"required"`
}

func (c *Commit) PreCommits() Votes { return c.preCommits }

// CopyCommit creates a deep copy of the commit info to prevent side effects from
// modifying a header variable.
func CopyCommit(commit *Commit) *Commit {
	cpy := *commit
	cpy.preCommits = make(Votes, len(commit.preCommits))
	copy(cpy.preCommits, commit.preCommits)
	return &cpy
}

// Evidence is the information that is used to decide the case.
type Evidence interface {
	Summary() Hash
	FactFinder() Address
}

// Conviction is the verdict that usually results when a validator
// finds other validator guilty of protocol violation.
type Conviction struct {
	blockNumber *big.Int `json:"blockNumber" gencodec:"required"`
	perpetrator Address  `json:"perpetrator" gencodec:"required"`
	evidence    Evidence `json:"evidence"    gencodec:"required"`
}

// Category returns the conviction category.
func (c *Conviction) Category() string {
	return reflect.TypeOf(c.evidence).Name()
}

// Convictions is a Conviction slice type for basic sorting.
type Convictions []*Conviction

// Len returns the length of s.
func (c Convictions) Len() int { return len(c) }

// Swap swaps the i'th and the j'th element in s.
func (c Convictions) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (c Convictions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(c[i])
	return enc
}

var _ Evidence = (*DuplicateVoting)(nil)

// DuplicateVoting satisfies the Evidence interface.
type DuplicateVoting struct {
	Vote1 *Vote `json:"vote1" gencodec:"required"`
	Vote2 *Vote `json:"vote2" gencodec:"required"`

	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

// Summary returns DuplicateVoting Hash.
func (dv *DuplicateVoting) Summary() Hash {
	return rlpHash(dv)
}

// FactFinder returns the validator responsible for gathering the facts.
func (dv *DuplicateVoting) FactFinder() Address {
	// @TODO (rgeraldes) - returns the validator addr based on the signature
	return Address{}
}

// StorageSize is a wrapper around a float value that supports user friendly
// formatting.
type StorageSize float64

// String implements the stringer interface.
func (s StorageSize) String() string {
	if s > 1000000 {
		return fmt.Sprintf("%.2f mB", s/1000000)
	} else if s > 1000 {
		return fmt.Sprintf("%.2f kB", s/1000)
	} else {
		return fmt.Sprintf("%.2f B", s)
	}
}

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (s StorageSize) TerminalString() string {
	if s > 1000000 {
		return fmt.Sprintf("%.2fmB", s/1000000)
	} else if s > 1000 {
		return fmt.Sprintf("%.2fkB", s/1000)
	} else {
		return fmt.Sprintf("%.2fB", s)
	}
}
