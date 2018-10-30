package crypto

import "errors"

var (
	// ErrInvalidSig signals invalid signature signature values.
	ErrInvalidSig = errors.New("invalid v, r, s values")
)
