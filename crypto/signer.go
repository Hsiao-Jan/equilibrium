package crypto

import (
	"math/big"
)

// Hasher represents a type capable of creating an hash.
type Hasher interface {
	HashWithData(data ...interface{}) Hash
}

// SignedType is a type whose integrity cannot be compromised without being detected.
type SignedType interface {
	Hasher
	Protected() bool
	ChainID() *big.Int
	SignatureValues() (R, S, V *big.Int)
}

// Signer encapsulates the signature handling.
type Signer interface {
	// Sender returns the sender address of the transaction.
	//Sender(typ SignedType) (accounts.Address, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(sig []byte) (r, s, v *big.Int, err error)
	// Hash returns the hash to be signed.
	Hash(h Hasher) Hash
	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}
