package chain

import (
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
	"github.com/kowala-tech/equilibrium/state/accounts/transaction"
)

// @TODO (rgeraldes)
// RemovedLogsEvent is posted when a reorg happens.
type RemovedLogsEvent struct{ Logs []*transaction.Log }

// @TODO description
type ChainEvent struct {
	Block *types.Block
	Hash  crypto.Hash
	Logs  []*transaction.Log
}

// @TODO description
type ChainHeadEvent struct{ Block *types.Block }
