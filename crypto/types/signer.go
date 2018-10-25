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
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/crypto"
	miningtypes "github.com/kowala-tech/equilibrium/services/mining/types"
	"github.com/kowala-tech/equilibrium/types"
)

var (
	errInvalidSig     = errors.New("invalid v, r, s values")
	errInvalidChainID = errors.New("invalid chain id for signer")
)

// @TODO (rgeraldes) - is equal necessary?
// @TODO (rgeraldes) - review comments

type Hasher interface {
	HashWithData(data ...interface{}) types.Hash
}

type Sender interface {
	Hasher
	Protected() bool
	ChainID() *big.Int
	SignatureValues() (R, S, V *big.Int)
}

// Signer encapsulates the signature handling.
type Signer interface {
	// Sender returns the sender address of the transaction.
	Sender(s Sender) (types.Address, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(sig []byte) (r, s, v *big.Int, err error)
	// Hash returns the hash to be signed.
	Hash(h Hasher) types.Hash
	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}

// MakeSigner returns a Signer based on the given chain config and block number.
func MakeSigner( /*cfg *params.ChainConfig, blockNumber *big.Int*/ ) Signer {
	return NewProductionSigner(cfg.ChainID)
}

// ProductionSigner represents a signer that prevents replay attacks.
type ProductionSigner struct {
	chainID    *big.Int
	chainIDMul *big.Int
}

// NewProductionSigner builds a sensible production Signer.
func NewProductionSigner(chainID *big.Int) *ProductionSigner {
	signer := &ProductionSigner{
		chainID:    new(big.Int),
		chainIDMul: new(big.Int),
	}
	if chainID != nil {
		signer.chainID.Set(chainID)
		signer.chainIDMul.Mul(signer.chainID, big.NewInt(2))
	}

	return signer
}

func (s ProductionSigner) Equal(s2 Signer) bool {
	andromeda, ok := s2.(ProductionSigner)
	return ok && andromeda.chainID.Cmp(s.chainID) == 0
}

func (s ProductionSigner) Sender(sn Sender) (types.Address, error) {
	if !sn.Protected() {
		return UnsafeSigner{}.Sender(sn)
	}
	if sn.ChainID().Cmp(s.chainID) != 0 {
		return Address{}, errInvalidChainID
	}

	snR, snS, snV := sn.SignatureValues()

	V := new(big.Int).Sub(snV, s.chainIDMul)
	V.Sub(V, common.Big8)
	return recoverPlain(s.Hash(sn), snR, snS, V, true)
}

// SignatureValues returns a new signature. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s ProductionSigner) SignatureValues(sig []byte) (R, S, V *big.Int, err error) {
	R, S, V, err = UnsafeSigner{}.SignatureValues(sig)
	if err != nil {
		return nil, nil, nil, err
	}
	if s.chainID.Sign() != 0 {
		V = big.NewInt(int64(sig[64] + 35))
		V.Add(V, s.chainIDMul)
	}
	return R, S, V, nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s ProductionSigner) Hash(h Hasher) types.Hash {
	return h.HashWithData(s.chainID, uint(0), uint(0))
}

// UnsafeSigner does not provide replay attack protection.
// NOTE: use only for tests
type UnsafeSigner struct{}

func (s UnsafeSigner) Equal(s2 Signer) bool {
	_, ok := s2.(UnsafeSigner)
	return ok
}

func (s UnsafeSigner) SignatureValues(sig []byte) (sr, ss, sv *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	sr = new(big.Int).SetBytes(sig[:32])
	ss = new(big.Int).SetBytes(sig[32:64])
	sv = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return
}

func (s UnsafeSigner) Sender(sn Sender) (types.Address, error) {
	snR, snS, snV := sn.SignatureValues()
	return recoverPlain(s.Hash(sn), snR, snS, snV, true)
}

func (s UnsafeSigner) Hash(h Hasher) Hash {
	return h.HashWithData()
}

func recoverPlain(sighash Hash, R, S, Vb *big.Int, homestead bool) (types.Address, error) {
	if Vb.BitLen() > 8 {
		return Address{}, errInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return Address{}, errInvalidSig
	}
	// encode the snature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the signature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return Address{}, errors.New("invalid public key")
	}
	var addr Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

// SignTx signs the transaction using the given signer and private key
func SignTx(tx *Transaction, signer Signer, prv *ecdsa.PrivateKey) (*types.Transaction, error) {
	h := signer.Hash(tx)
	sig, err := crypto.Sign(h.Bytes(), prv)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(signer, sig)
}

// SignProposal signs the proposal using the given signer and private key
func SignProposal(proposal *Proposal, signer Signer, prv *ecdsa.PrivateKey) (*miningtypes.Proposal, error) {
	h := signer.Hash(proposal)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return proposal.WithSignature(signer, sig)

}

// SignVote signs the vote using the given signer and private key
func SignVote(vote *Vote, signer Signer, prv *ecdsa.PrivateKey) (*miningtypes.Vote, error) {
	h := signer.Hash(vote)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return vote.WithSignature(signer, sig)
}

// sigCache is used to cache the derived sender and contains
// the signer used to derive it.
type sigCache struct {
	signer Signer
	from   Address
}

func TxSender(signer Signer, tx *Transaction) (types.Address, error) {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.Sender(tx)
	if err != nil {
		return Address{}, err
	}
	tx.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

func ProposalSender(signer Signer, proposal *Proposal) (types.Address, error) {
	if sc := proposal.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.Sender(proposal)
	if err != nil {
		return Address{}, err
	}
	proposal.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

func VoteSender(signer Signer, vote *Vote) (types.Address, error) {
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
		return Address{}, err
	}
	vote.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}
