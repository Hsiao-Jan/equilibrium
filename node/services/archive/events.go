package archive

import (
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
	"github.com/kowala-tech/equilibrium/state/accounts/transaction"
)

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []*transaction.Transaction }

// PendingLogsEvent is posted pre mining and notifies of pending logs.
type PendingLogsEvent struct {
	Logs []*transaction.Log
}

// PendingStateEvent is posted pre mining and notifies of pending state changes.
type PendingStateEvent struct{}

//// NewMinedBlockEvent is posted when a block has been imported.
//type NewMinedBlockEvent struct{ Block *types.Block }

// RemovedLogsEvent is posted when a reorg happens
type RemovedLogsEvent struct{ Logs []*transaction.Log }

type ChainEvent struct {
	Block *types.Block
	Hash  crypto.Hash
	Logs  []*transaction.Log
}

type ChainSideEvent struct {
	Block *types.Block
}

type ChainHeadEvent struct{ Block *types.Block }
