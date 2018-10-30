package signer

import (
	"math/big"

	"github.com/kowala-tech/equilibrium/network/params"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/state/accounts"
)

// ProductionSigner represents a signer that prevents replay attacks.
type ProductionSigner struct {
	chainID    *big.Int
	chainIDMul *big.Int
}

// NewProductionSigner builds a sensible production Signer.
func NewProductionSigner() *ProductionSigner {
	signer := &ProductionSigner{
		chainID:    params.ChainID,
		chainIDMul: new(big.Int),
	}
	signer.chainIDMul.Mul(signer.chainID, big.NewInt(2))

	return signer
}

func (s ProductionSigner) Equal(s2 Signer) bool {
	andromeda, ok := s2.(ProductionSigner)
	return ok && andromeda.chainID.Cmp(s.chainID) == 0
}

func (s ProductionSigner) Sender(sn Sender) (accounts.Address, error) {
	if !sn.Protected() {
		return UnsafeSigner{}.Sender(sn)
	}
	if sn.ChainID().Cmp(s.chainID) != 0 {
		return accounts.Address{}, ErrInvalidChainID
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
func (s ProductionSigner) Hash(h Hasher) crypto.Hash {
	return h.HashWithData(s.chainID, uint(0), uint(0))
}

/*
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
*/
