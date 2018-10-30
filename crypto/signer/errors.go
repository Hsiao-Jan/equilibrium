package signer

import "errors"

var (
	// ErrInvalidChainID signals an invalid chain id.
	ErrInvalidChainID = errors.New("invalid chain id for signer")
)
