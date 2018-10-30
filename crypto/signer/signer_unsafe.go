package signer

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/state/accounts"
)

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

func (s UnsafeSigner) Sender(sn Sender) (accounts.Address, error) {
	snR, snS, snV := sn.SignatureValues()
	return recoverPlain(s.Hash(sn), snR, snS, snV, true)
}

func (s UnsafeSigner) Hash(h Hasher) crypto.Hash {
	return h.HashWithData()
}

func recoverPlain(sighash crypto.Hash, R, S, Vb *big.Int, homestead bool) (accounts.Address, error) {
	if Vb.BitLen() > 8 {
		return accounts.Address{}, crypto.ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return accounts.Address{}, crypto.ErrInvalidSig
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
		return accounts.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return accounts.Address{}, errors.New("invalid public key")
	}
	var addr accounts.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}
