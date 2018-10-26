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
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
	miningTypes "github.com/kowala-tech/equilibrium/node/services/mining/types"
	"github.com/kowala-tech/equilibrium/state/accounts"
	"github.com/kowala-tech/equilibrium/state/accounts/transaction"
	"github.com/kowala-tech/equilibrium/state/trie"
)

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go
//go:generate gencodec -type Commit -out gen_commit_json.go

var (
	// EmptyRootHash represents a trie hash for an empty slice of transactions.
	EmptyRootHash = trie.DeriveSha(transaction.Transactions{})
	// EmptyHash represents the rlp hash for nil.
	EmptyHash = crypto.RLPHash(nil)
)

// Header represents a block header.
type Header struct {
	// basic info
	Number            *big.Int    `json:"number"            gencodec:"required"`
	PreviousBlockHash crypto.Hash `json:"previousBlockHash" gencodec:"required"`
	Extra             []byte      `json:"extraData"         gencodec:"required"`

	// consensus
	Snapshot crypto.Hash      `json:"stateRoot" gencodec:"required"`
	Time     *big.Int         `json:"timestamp" gencodec:"required"` // time is used to sync the validators upon a new consensus round.
	Proposer accounts.Address `json:"proposer"  gencodec:"required"`

	// block data
	LastCommitHash        crypto.Hash  `json:"lastCommitRoot"  gencodec:"required"`
	ProtocolViolationHash crypto.Hash  `json:"violationsHash"  gencodec:"required"`
	TxHash                crypto.Hash  `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash           crypto.Hash  `json:"receiptHash"      gencodec:"required"`
	Bloom                 common.Bloom `json:"logsBloom"        gencodec:"required"`
}

// headerMarshaling field type overrides for gencodec
type headerMarshaling struct {
	Number *hexutil.Big
	Extra  hexutil.Bytes
	Time   *hexutil.Big
	Hash   crypto.Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() crypto.Hash {
	return crypto.RLPHash(h)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Number.BitLen()+h.Time.BitLen())/8)
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
	protocolViolations []*miningTypes.Conviction
	transactions       transaction.Transactions

	// caches
	hash atomic.Value
	size atomic.Value
}

// NewBlock creates a new block. The values of TxHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs and receipts.
func NewBlock(header *Header, txs []*transaction.Transaction, receipts []*transaction.Receipt, lastCommit *Commit, violations []*miningTypes.Conviction) (*Block, error) {
	if len(txs) != len(receipts) {
		return nil, fmt.Errorf("Number of transactions (%d) does not match number of receipts (%d)", len(txs), len(receipts))
	}

	block := &Block{header: CopyHeader(header)}

	if len(txs) == 0 {
		block.header.TxHash = EmptyRootHash
	} else {
		block.header.TxHash = trie.DeriveSha(transaction.Transactions(txs))
		block.transactions = make(transaction.Transactions, len(txs))
		copy(block.transactions, txs)
	}

	if len(receipts) == 0 {
		block.header.ReceiptHash = EmptyRootHash
	} else {
		block.header.ReceiptHash = trie.DeriveSha(transaction.Receipts(receipts))
		block.header.Bloom = CreateBloom(receipts)
	}

	if lastCommit != nil {
		block.header.LastCommitHash = crypto.RLPHash(lastCommit)
		block.lastCommit = CopyCommit(lastCommit)
	}

	if len(violations) == 0 {
		block.header.ProtocolViolationHash = EmptyHash
	} else {
		block.header.ProtocolViolationHash = crypto.RLPHash(violations)
		block.protocolViolations = make([]*miningTypes.Conviction, len(violations))
		copy(block.protocolViolations, violations)
	}

	return block, nil
}

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *Block) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := common.WriteCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() crypto.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(crypto.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents together.
type Body struct {
	Transactions       []*transaction.Transaction
	LastCommit         *Commit
	ProtocolViolations []*miningTypes.Conviction
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
func (b *Block) WithBody(txs []*transaction.Transaction, lastCommit *Commit, protocolViolations []*miningTypes.Conviction) *Block {
	block := &Block{
		header:             CopyHeader(b.header),
		transactions:       make([]*transaction.Transaction, len(txs)),
		protocolViolations: make([]*miningTypes.Conviction, len(protocolViolations)),
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
func (b *Block) Transactions() transaction.Transactions { return b.transactions }

// LastCommit returns the list of pre-commits for the previous block.
func (b *Block) LastCommit() *Commit { return CopyCommit(b.lastCommit) }

// ProtocolViolations returns the list of convictions.
func (b *Block) ProtocolViolations() []*miningTypes.Conviction { return b.protocolViolations }

// Transaction returns a transaction for a given hash if the transaction
// is present in the block.
func (b *Block) Transaction(hash crypto.Hash) *transaction.Transaction {
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
func (b *Block) PreviousBlockHash() crypto.Hash { return b.header.PreviousBlockHash }

// Extra returns extra information present in the block.
func (b *Block) Extra() []byte { return common.CopyBytes(b.header.Extra) }

// Time returns
func (b *Block) Time() *big.Int { return new(big.Int).Set(b.header.Time) }

// Proposer returns the validator responsible for proposing the block.
func (b *Block) Proposer() accounts.Address { return b.header.Proposer }

// LastCommitHash returns the hash of the +2/3 precommit signatures for the previous block.
func (b *Block) LastCommitHash() crypto.Hash { return b.header.LastCommitHash }

// ProtocolViolationHash returns the hash of the protocol violations.
func (b *Block) ProtocolViolationHash() crypto.Hash { return b.header.ProtocolViolationHash }

// Snapshot returns the block's state root.
func (b *Block) Snapshot() crypto.Hash { return b.header.Snapshot }

// Bloom returns the logs bloom filter.
func (b *Block) Bloom() common.Bloom { return b.header.Bloom }

// TxHash returns the transactions' trie root.
func (b *Block) TxHash() crypto.Hash { return b.header.TxHash }

// ReceiptHash returns the receipts' trie root.
func (b *Block) ReceiptHash() crypto.Hash { return b.header.ReceiptHash }

// "external" block encoding.
type extblock struct {
	Header             *Header
	Transactions       []*transaction.Transaction
	LastCommit         *Commit
	ProtocolViolations []*miningTypes.Conviction
}

// DecodeRLP decodes the block.
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.transactions, b.lastCommit, b.protocolViolations = eb.Header, eb.Transactions, eb.LastCommit, eb.ProtocolViolations
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
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
	preCommits miningTypes.Votes `json:"preCommits" gencodec:"required"`
}

// PreCommits returns the validators' pre commits.
func (c *Commit) PreCommits() miningTypes.Votes { return c.preCommits }

// CopyCommit creates a deep copy of the commit info to prevent side effects from
// modifying a header variable.
func CopyCommit(commit *Commit) *Commit {
	cpy := *commit
	cpy.preCommits = make(miningTypes.Votes, len(commit.preCommits))
	copy(cpy.preCommits, commit.preCommits)
	return &cpy
}

func CreateBloom(receipts transaction.Receipts) common.Bloom {
	bin := new(big.Int)
	for _, receipt := range receipts {
		bin.Or(bin, LogsBloom(receipt.Logs))
	}

	return common.BytesToBloom(bin.Bytes())
}

func LogsBloom(logs []*transaction.Log) *big.Int {
	bin := new(big.Int)
	for _, log := range logs {
		bin.Or(bin, common.Bloom9(log.ContractAddress.Bytes()))
		for _, b := range log.Topics {
			bin.Or(bin, common.Bloom9(b[:]))
		}
	}

	return bin
}
