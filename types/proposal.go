package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
)

//go:generate gencodec -type proposaldata -field-override proposaldataMarshalling -out gen_proposal_json.go
//go:generate gencodec -type Metadata -field-override MetadataMarshalling -out gen_metadata_json.go
//go:generate gencodec -type Chunk -field-override chunkMarshalling -out gen_chunk_json.go

// Proposal represents a consensus block proposal.
type Proposal struct {
	data proposaldata

	// caches
	hash atomic.Value
	size atomic.Value // @TODO (rgeraldes) - confirm if it's necessary
	from atomic.Value
}

type proposaldata struct {
	BlockNumber   *big.Int  `json:"blockNumber"   gencodec:"required"`
	Round         uint64    `json:"round"         gencodec:"required"`
	LockedRound   uint64    `json:"lockedRound"   gencodec:"required"`
	LockedBlock   Hash      `json:"lockedBlock"   gencodec:"required"`
	BlockMetadata *Metadata `json:"metadata"      gencodec:"required"`
	//Timestamp     time.Time      `json:"time"		gencoded:"required"` // @TODO(rgeraldes) confirm if it's necessary

	// signature values
	V *big.Int `json:"v"      gencodec:"required"`
	R *big.Int `json:"r"      gencodec:"required"`
	S *big.Int `json:"s"      gencodec:"required"`
}

// proposaldataMarshalling - field type overrides for gencodec
type proposaldataMarshalling struct {
	BlockNumber *hexutil.Big
	Round       hexutil.Uint64
	LockedRound hexutil.Uint64
	V           *hexutil.Big
	R           *hexutil.Big
	S           *hexutil.Big
}

// NewProposal returns a new proposal
func NewProposal(blockNumber *big.Int, round uint64, blockMetadata *Metadata, lockedRound int, lockedBlock Hash) *Proposal {
	return newProposal(blockNumber, round, blockMetadata, lockedRound, lockedBlock)
}

func newProposal(blockNumber *big.Int, round uint64, blockMetadata *Metadata, lockedRound int, lockedBlock Hash) *Proposal {
	d := proposaldata{
		BlockNumber:   new(big.Int),
		BlockMetadata: blockMetadata,
		Round:         round,
		V:             new(big.Int),
		R:             new(big.Int),
		S:             new(big.Int),
	}

	if blockNumber != nil {
		d.BlockNumber.Set(blockNumber)
	}

	return &Proposal{data: d}
}

// EncodeRLP implements rlp.Encoder
func (prop *Proposal) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &prop.data)
}

// DecodeRLP implements rlp.Decoder
func (prop *Proposal) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&prop.data)
	if err == nil {
		prop.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

func (prop *Proposal) BlockNumber() *big.Int { return prop.data.BlockNumber }
func (prop *Proposal) Round() uint64         { return prop.data.Round }
func (prop *Proposal) LockedRound() uint64   { return prop.data.LockedRound }
func (prop *Proposal) LockedBlock() Hash     { return prop.data.LockedBlock }
func (prop *Proposal) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return prop.data.R, prop.data.S, prop.data.V
}
func (prop *Proposal) BlockMetadata() *Metadata { return prop.data.BlockMetadata }

//func (p *Proposal) Timestamp() time.Time          { return p.data.Timestamp }

// Hash hashes the RLP encoding of the proposal.
// It uniquely identifies the proposal.
func (prop *Proposal) Hash() Hash {
	if hash := prop.hash.Load(); hash != nil {
		return hash.(Hash)
	}
	v := rlpHash(prop)
	prop.hash.Store(v)
	return v
}

// ProtectedHash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (prop *Proposal) ProtectedHash(chainID *big.Int) Hash {
	return prop.HashWithData(chainID, uint(0), uint(0))
}

func (prop *Proposal) HashWithData(data ...interface{}) Hash {
	propData := []interface{}{
		prop.data.BlockNumber,
		prop.data.Round,
		prop.data.BlockMetadata,
		prop.data.LockedRound,
		prop.data.LockedBlock,
	}
	return rlpHash(append(propData, data...))
}

// Size returns the proposal size
func (prop *Proposal) Size() StorageSize {
	if size := prop.size.Load(); size != nil {
		return size.(StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &prop.data)
	prop.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// WithSignature returns a new proposal with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (proposal *Proposal) WithSignature(signer Signer, sig []byte) (*Proposal, error) {
	r, s, v, err := signer.SignatureValues(sig)
	if err != nil {
		return nil, err
	}

	cpy := &Proposal{data: proposal.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v

	return cpy, nil
}

func (proposal *Proposal) Protected() bool {
	return true
}

func (proposal *Proposal) ChainID() *big.Int {
	return deriveChainID(proposal.data.V)
}

func (proposal *Proposal) SignatureValues() (R, S, V *big.Int) {
	R, S, V = proposal.data.R, proposal.data.S, proposal.data.V
	return
}

// @TODO (rgeraldes) - add metadata & timestamp
func (prop *Proposal) String() string {
	enc, _ := rlp.EncodeToBytes(&prop.data)
	return fmt.Sprintf(`
	Proposal(%x)
	Block Number:		%v
	Round:	  			%d
	Locked Block:		%x
	Locked Round:		%d
	V:        			%#x
	R:        			%#x
	S:        			%#x
	Hex:      			%x
`,
		prop.Hash(),
		prop.data.BlockNumber,
		prop.data.Round,
		prop.data.LockedBlock,
		prop.data.LockedRound,
		prop.data.V,
		prop.data.R,
		prop.data.S,
		enc,
	)
}

// @TODO (rgeraldes) - review uint64/int

// @TODO (rgeraldes) - move to another place
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// Chunk represents a fragment of information
type Chunk struct {
	Index uint64 `json:"index"  gencodec:"required"`
	Data  []byte `json:"bytes"  gencodec:"required"`
	Proof Hash   `json:"proof"  gencodec:"required"`
}

type chunkMarshalling struct {
	Index hexutil.Uint64
	Data  hexutil.Bytes
}

// DataSet represents content as a set of data chunks
type DataSet struct {
	meta *Metadata

	count      uint      // number of current data chunks
	data       []*Chunk  // stores data chunks
	membership *BitArray // indicates whether a data unit is present or not
	l          sync.RWMutex
}

// Metadata represents the content specifications
type Metadata struct {
	NChunks uint `json:"nchunks" gencodec:"required"`
	Root    Hash `json:"proof"   gencodec:"required"` // root hash of the trie
}

type MetadataMarshalling struct {
	NChunks hexutil.Uint64
}

func NewDataSetFromMeta(meta *Metadata) *DataSet {
	return &DataSet{
		meta:       CopyMeta(meta),
		count:      0,
		membership: common.NewBitArray(uint64(meta.NChunks)),
		data:       make([]*Chunk, meta.NChunks),
	}
}

func CopyMeta(meta *Metadata) *Metadata {
	cpy := *meta
	return &cpy
}

func NewDataSetFromData(data []byte, size int) *DataSet {
	total := (len(data) + size - 1) / size
	chunks := make([]*Chunk, total)
	membership := common.NewBitArray(uint64(total))
	for i := 0; i < total; i++ {
		chunk := &Chunk{
			Index: uint64(i),
			Data:  data[i*size : min(len(data), (i+1)*size)],
			// @NOTE (rgeraldes) - this is temporary workaround.
			// This is necessary for now because the fragments are not sent to peers
			// if the data chunk doesn't have a unique summary. A repeated request is ignored.
			Proof: rlpHash(data[i*size : min(len(data), (i+1)*size)]),
		}
		chunks[i] = chunk
		membership.Set(i)
	}

	// @TODO (rgeraldes)
	// compute merkle proofs
	//trie := new(trie.Trie)
	//trie.Update()
	//root := trie.Hash()

	return &DataSet{
		meta: &Metadata{
			NChunks: uint(total),
			Root:    common.Hash{},
		},
		data:       chunks,
		membership: membership,
		count:      uint(total),
	}
}

func (ds *DataSet) Metadata() *Metadata {
	return ds.meta
}

func (ds *DataSet) Size() uint {
	return ds.meta.NChunks
}

func (ds *DataSet) Count() uint {
	ds.l.RLock()
	defer ds.l.RUnlock()
	return ds.count
}

func (ds *DataSet) Get(i int) *Chunk {
	// @TODO (rgeraldes) - add logic to verify if the fragment
	// exists

	ds.l.RLock()
	defer ds.l.RUnlock()
	return ds.data[i]
}

func (ds *DataSet) Add(chunk *Chunk) error {
	if chunk == nil {
		return errors.New("got a nil fragment")
	}

	ds.l.Lock()

	// @TODO (rgeraldes) - validate index
	// @TODO (rgeraldes) - check hash proof
	ds.data[chunk.Index] = chunk
	// @TODO (rgeraldes) - review int vs uint64
	ds.membership.Set(int(chunk.Index))
	ds.count++

	ds.l.Unlock()

	return nil
}

func (ds *DataSet) HasAll() bool {
	ds.l.RLock()
	defer ds.l.RUnlock()
	return ds.count == ds.meta.NChunks
}

func (ds *DataSet) Data() []byte {
	ds.l.RLock()
	defer ds.l.RUnlock()

	var buffer bytes.Buffer
	for _, chunk := range ds.data {
		buffer.Write(chunk.Data)
	}
	return buffer.Bytes()
}

func (ds *DataSet) Assemble() (*Block, error) {
	ds.l.RLock()
	defer ds.l.RUnlock()

	var block Block
	if err := rlp.DecodeBytes(ds.Data(), &block); err != nil {
		return nil, err
	}
	return &block, nil
}
