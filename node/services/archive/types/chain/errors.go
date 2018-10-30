package chain

import "errors"

var (
	// ErrKnownBlock is returned when a block to import is already known locally.
	ErrKnownBlock = errors.New("block already known")

	ErrNoGenesis = errors.New("Genesis not found in chain")
)
