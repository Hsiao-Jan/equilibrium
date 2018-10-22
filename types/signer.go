package types

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/kcoin/client/params"
)

// @TODO (rgeraldes) - is equal necessary?
// @TODO (rgeraldes) - review comments

type Hasher interface {
	HashWithData(data ...interface{}) Hash
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
	Sender(s Sender) (Address, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(sig []byte) (r, s, v *big.Int, err error)
	// Hash returns the hash to be signed.
	Hash(h Hasher) Hash
	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}

// MakeSigner returns a Signer based on the given chain config and block number.
func MakeSigner(cfg *params.ChainConfig, blockNumber *big.Int) Signer {
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
}

func (s ProductionSigner) Equal(s2 Signer) bool {
	andromeda, ok := s2.(ProductionSigner)
	return ok && andromeda.chainID.Cmp(s.chainID) == 0
}

func (s ProductionSigner) Sender(sn Sender) (Address, error) {
	if !sn.Protected() {
		return UnsafeSigner{}.Sender(sn)
	}
	if sn.ChainID().Cmp(s.chainID) != 0 {
		return Address{}, ErrInvalidChainID
	}

	snR, snS, snV := sn.SignatureValues()

	V := new(big.Int).Sub(snV, s.chainIDMul)
	V.Sub(V, common.Big8)
	return recoverPlain(s.Hash(sn), snR, snS, V, true)
}

// SignatureValues returns a new signature. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s ProductionSigner) SignatureValues(sig []byte) (R, S, V *big.Int, err error) {
	R, S, V, err = UnprotectedSigner{}.SignatureValues(sig)
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
func (s ProductionSigner) Hash(h Hasher) common.Hash {
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

func (s UnsafeSigner) Sender(sn Sender) (Address, error) {
	snR, snS, snV := sn.SignatureValues()
	return recoverPlain(s.Hash(sn), snR, snS, snV, true)
}

func (s UnsafeSigner) Hash(h Hasher) Hash {
	return h.HashWithData()
}

func recoverPlain(sighash Hash, R, S, Vb *big.Int, homestead bool) (Address, error) {
	if Vb.BitLen() > 8 {
		return Address{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return Address{}, ErrInvalidSig
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

// deriveChainID derives the chain id from the given v parameter
func deriveChainID(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}
