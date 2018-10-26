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
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
	"github.com/kowala-tech/equilibrium/state/accounts"
)

//go:generate gencodec -type voteData -field-override voteDataMarshaling -out gen_vote_json.go

// VoteType represents the different kinds of consensus votes.
type VoteType byte

const (
	// PreVote represents a vote in the first consensus sub-election.
	PreVote VoteType = iota

	// PreCommit represents a vote in the second consensus sub-election.
	PreCommit
)

// Vote represents a consensus vote.
type Vote struct {
	data voteData

	hash atomic.Value
	from atomic.Value
	size atomic.Value
}

type voteData struct {
	// BlockNumber represents the block number under voting.
	BlockNumber *big.Int `json:"blockNumber" gencodec:"required"`

	// Round represents the current consensus voting round.
	Round uint64 `json:"round" gencodec:"required"`

	// Type is either a pre-vote or a pre-commit.
	Type VoteType `json:"type" gencodec:"required"`

	// Decision represents an individual's choice for or against some block.
	Decision crypto.Hash `json:"blockHash" gencodec:"required"`

	// Signature values.
	V *big.Int `json:"v"   gencodec:"required"`
	R *big.Int `json:"r"   gencodec:"required"`
	S *big.Int `json:"s"   gencodec:"required"`
}

// voteDataMarshaling - field type overrides for gencodec
type voteDataMarshaling struct {
	BlockNumber *hexutil.Big
	Round       hexutil.Uint64
	V           *hexutil.Big
	R           *hexutil.Big
	S           *hexutil.Big
}

func NewVote(blockNumber *big.Int, round uint64, decision crypto.Hash, voteType VoteType) *Vote {
	return newVote(blockNumber, round, decision, voteType)
}

func newVote(blockNumber *big.Int, round uint64, decision crypto.Hash, voteType VoteType) *Vote {
	d := voteData{
		BlockNumber: new(big.Int),
		Round:       round,
		Type:        voteType,
		Decision:    decision,
		V:           new(big.Int),
		R:           new(big.Int),
		S:           new(big.Int),
	}

	if blockNumber != nil {
		d.BlockNumber.Set(blockNumber)
	}

	return &Vote{data: d}
}

// EncodeRLP implements rlp.Encoder
func (vote *Vote) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &vote.data)
}

// DecodeRLP implements rlp.Decoder
func (vote *Vote) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&vote.data)
	if err == nil {
		vote.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// BlockNumber returns the block number under voting.
func (vote *Vote) BlockNumber() *big.Int { return vote.data.BlockNumber }

// Decision returns the individual's choice.
func (vote *Vote) Decision() crypto.Hash { return vote.data.Decision }

// Round returns the respective consensus round.
func (vote *Vote) Round() uint64 { return vote.data.Round }

// Type returns the vote type (pre-vote/pre-commit).
func (vote *Vote) Type() VoteType { return vote.data.Type }

// Size returns the true RLP encoded storage size of the vote, either by
// encoding and returning it, or returning a previsouly cached value.
func (vote *Vote) Size() common.StorageSize {
	if size := vote.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := common.WriteCounter(0)
	rlp.Encode(&c, &vote.data)
	vote.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// Hash hashes the RLP encoding of the vote. It uniquely identifies the vote.
func (vote *Vote) Hash() crypto.Hash {
	if hash := vote.hash.Load(); hash != nil {
		return hash.(crypto.Hash)
	}
	v := crypto.RLPHash(vote)
	vote.hash.Store(v)
	return v
}

// HashWithData returns the vote hash to be signed by the sender.
func (vote *Vote) HashWithData(data ...interface{}) crypto.Hash {
	voteData := []interface{}{
		vote.data.BlockNumber,
		vote.data.Round,
		vote.data.Type,
		vote.data.Decision,
	}
	return crypto.RLPHash(append(voteData, data...))
}

// WithSignature returns a new vote with the given signature.
func (vote *Vote) WithSignature(signer crypto.Signer, sig []byte) (*Vote, error) {
	r, s, v, err := signer.SignatureValues(sig)
	if err != nil {
		return nil, err
	}

	cpy := &Vote{data: vote.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v

	return cpy, nil
}

// Protected specifies whether the vote is protected from replay attacks or not.
func (vote *Vote) Protected() bool { return true }

// ChainID derives the vote chain ID from the signature.
func (vote *Vote) ChainID() *big.Int { return common.DeriveChainID(vote.data.V) }

// SignatureValues returns the vote raw signature values.
func (vote *Vote) SignatureValues() (R, S, V *big.Int) {
	R, S, V = vote.data.R, vote.data.S, vote.data.V
	return
}

// String presents the vote values.
func (vote *Vote) String() string {
	enc, _ := rlp.EncodeToBytes(&vote.data)
	return fmt.Sprintf(`
	Vote(%x)
	Block Number:		%v
	Round:	  			%d
	Type: 				%v
	Decision:			%x
	V:        			%#x
	R:        			%#x
	S:        			%#x
	Hex:      			%x
`,
		vote.Hash(),
		vote.data.BlockNumber,
		vote.data.Round,
		vote.data.Type,
		vote.data.Decision,
		vote.data.V,
		vote.data.R,
		vote.data.S,
		enc,
	)
}

// Votes represents a slice of votes.
type Votes []*Vote

// AddressVote is a wrapper type that includes the address.
type AddressVote interface {
	Address() accounts.Address
	Vote() *Vote
}

type addressVote struct {
	vote    *Vote
	address accounts.Address
}

// NewAddressVote derives the sender and includes it in the AddressVote with the vote.
func NewAddressVote(signer crypto.Signer, vote *Vote) (*addressVote, error) {
	address, err := VoteSender(signer, vote)
	if err != nil {
		return nil, err
	}

	return &addressVote{
		vote,
		address,
	}, nil
}

func (addressVote *addressVote) Address() accounts.Address {
	return addressVote.address
}

func (addressVote *addressVote) Vote() *Vote {
	return addressVote.vote
}

// @TODO - review

type VotesSet struct {
	m        map[crypto.Hash]*Vote // non-nil votes
	nilVotes map[crypto.Hash]*Vote // nil votes
	counter  map[crypto.Hash]int   // map[block.Hash]count
	leader   crypto.Hash           // block.Hash
	l        sync.RWMutex
}

func NewVotesSet() *VotesSet {
	return &VotesSet{
		m:        make(map[crypto.Hash]*Vote),
		nilVotes: make(map[crypto.Hash]*Vote),
		counter:  make(map[crypto.Hash]int),
	}
}

func (v *VotesSet) Add(vote *Vote) {
	v.l.Lock()
	defer v.l.Unlock()

	if bytes.Equal(vote.data.Decision.Bytes(), crypto.Hash{}.Bytes()) {
		v.nilVotes[vote.Hash()] = vote
		return
	}

	//log.Debug("voting. add vote", "type", vote.data.Type, "number", vote.data.BlockNumber.String(),
	//	"round", vote.data.Round, "hash", vote.data.Decision.String())

	v.m[vote.Hash()] = vote
	v.counter[vote.data.Decision]++
	if v.counter[vote.data.Decision] > v.counter[v.leader] {
		v.leader = vote.data.Decision
	}
}

var (
	errNonNilDuplicate = errors.New("duplicate NON-NIL vote")
	errNilDuplicate    = errors.New("duplicate NIL vote")
)

func (v *VotesSet) Contains(h crypto.Hash) error {
	v.l.RLock()
	defer v.l.RUnlock()

	_, res := v.m[h]
	if res {
		return errNonNilDuplicate
	}

	if !res {
		_, res = v.nilVotes[h]
		if res {
			return errNilDuplicate
		}
	}

	return nil
}

func (v *VotesSet) Len() int {
	v.l.RLock()
	res := len(v.m)
	v.l.RUnlock()
	return res
}

func (v *VotesSet) Leader() crypto.Hash {
	v.l.RLock()
	res := v.leader
	v.l.RUnlock()
	return res
}

func (v *VotesSet) Get(h crypto.Hash) (*Vote, bool) {
	v.l.RLock()
	vote, ok := v.m[h]
	v.l.RUnlock()
	return vote, ok
}

func VoteSender(signer crypto.Signer, vote *Vote) (accounts.Address, error) {
	// @TODO (rgeraldes)
	/*
		if sc := vote.from.Load(); sc != nil {
			sigCache := sc.(sigCache)
			// If the signer used to derive from in a previous
			// call is not the same as used current, invalidate
			// the cache.
			if sigCache.signer.Equal(signer) {
				return sigCache.from, nil
			}
		}

		addr, err := signer.Sender(vote)
		if err != nil {
			return accounts.Address{}, err
		}
		vote.from.Store(sigCache{signer: signer, from: addr})
		return addr, nil
	*/
	return accounts.Address{}, nil
}
