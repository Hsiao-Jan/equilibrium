package crypto

import "errors"

var (
	// ErrInvalidSig signals invalid signature signature values.
	ErrInvalidSig = errors.New("invalid v, r, s values")

	// ErrInvalidChainID signals an invalid chain id.
	ErrInvalidChainID = errors.New("invalid chain id for signer")
)
