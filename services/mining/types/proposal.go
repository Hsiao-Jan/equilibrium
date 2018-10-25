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
	"sync/atomic"

	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
)

// Proposal represents a consensus block proposal.
type Proposal struct {
	data proposalData

	// caches
	hash atomic.Value
	from atomic.Value
}

type proposalData struct {
	// BlockNumber represents the block number under voting.
	BlockNumber *big.Int

	// Round represents the current consensus voting round.
	Round uint64

	// LockedRound represents the round in which there was a lock if there's an active lock.
	LockedRound *uint64

	// LockedBlock signals that a proposer is locked on a specific block if there's an active lock.
	LockedBlock Hash

	// Block represents the hash that uniquely identifies the proposed block.
	Block Hash

	// Signature values.
	V *big.Int
	R *big.Int
	S *big.Int
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

func NewProposal(blockNumber *big.Int, round uint64, lockedRound *uint64, lockedBlock, block Hash) *Proposal {
	return newProposal(blockNumber, round, lockedRound, lockedBlock, block)
}

func newProposal(blockNumber *big.Int, round uint64, lockedRound *uint64, lockedBlock, block Hash) *Proposal {
	data := proposaldata{
		BlockNumber: new(big.Int),
		Round:       round,
		LockedRound: lockedRound,
		LockedBlock: lockedBlock,
		Block:       block,
		V:           new(big.Int),
		R:           new(big.Int),
		S:           new(big.Int),
	}
	if blockNumber != nil {
		data.BlockNumber.Set(blockNumber)
	}

	return &Proposal{data: data}
}

// EncodeRLP implements rlp.Encoder.
func (prop *Proposal) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &prop.data)
}

// DecodeRLP implements rlp.Decoder.
func (p *Proposal) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	return s.Decode(&p.data)
}

// BlockNumber returns the block number under voting.
func (prop *Proposal) BlockNumber() *big.Int { return prop.data.BlockNumber }

// Round returns the respective consensus round.
func (prop *Proposal) Round() uint64 { return prop.data.Round }

// LockedRound returns the last round in which there was a lock if there's an active lock.
func (prop *Proposal) LockedRound() *uint64 { return prop.data.LockedRound }

// LockedBlock returns the unique identifier of the locked block if there's an active lock.
func (prop *Proposal) LockedBlock() Hash { return prop.data.LockedBlock }

// Block returns the hash that uniquely identifies the proposed block.
func (prop *Proposal) Block() Hash { return prop.data.Block }

// Hash hashes the RLP encoding of the proposal. It uniquely identifies the proposal.
func (prop *Proposal) Hash() Hash {
	if hash := prop.hash.Load(); hash != nil {
		return hash.(Hash)
	}
	v := rlpHash(prop)
	prop.hash.Store(v)
	return v
}

// HashWithData returns the proposal hash to be signed by the sender.
func (prop *Proposal) HashWithData(data ...interface{}) Hash {
	propData := []interface{}{
		prop.data.BlockNumber,
		prop.data.Round,
		prop.data.LockedRound,
		prop.data.LockedBlock,
		prop.data.Block,
	}
	return rlpHash(append(propData, data...))
}

// WithSignature returns a new proposal with the given signature.
func (proposal *Proposal) WithSignature(signer Signer, sig []byte) (*Proposal, error) {
	r, s, v, err := signer.SignatureValues(sig)
	if err != nil {
		return nil, err
	}

	cpy := &Proposal{data: proposal.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v

	return cpy, nil
}

// Protected specifies whether the proposal is protected from replay attacks or not.
func (proposal *Proposal) Protected() bool { return true }

// ChainID derives the proposal chain ID from the signature.
func (proposal *Proposal) ChainID() *big.Int { return deriveChainID(proposal.data.V) }

// SignatureValues returns the vote raw signature values
func (proposal *Proposal) SignatureValues() (R, S, V *big.Int) {
	R, S, V = proposal.data.R, proposal.data.S, proposal.data.V
}

// String presents the proposal values.
func (prop *Proposal) String() string {
	enc, _ := rlp.EncodeToBytes(&prop.data)
	return fmt.Sprintf(`
	Proposal(%x)
	Block Number:		%v
	Round:	  			%d
	Locked Round:		%d
	Locked Block:		%x
	Block: 				%x
	V:        			%#x
	R:        			%#x
	S:        			%#x
	Hex:      			%x
`,
		prop.Hash(),
		prop.data.BlockNumber,
		prop.data.Round,
		prop.data.LockedRound,
		prop.data.LockedBlock,
		prop.data.Block,
		prop.data.V,
		prop.data.R,
		prop.data.S,
		enc,
	)
}
