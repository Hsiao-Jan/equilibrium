package mining

import (
	"math/big"

	"github.com/kowala-tech/kcoin/client/common"
	"github.com/tendermint/tendermint/types"
)

// NewVoteEvent is posted when a consensus validator votes.
type NewVoteEvent struct{ Vote *types.Vote }

// NewProposalEvent is posted when a consensus validator proposes a new block.
type NewProposalEvent struct{ Proposal *types.Proposal }

// NewBlockFragmentEvent is posted when a consensus validator broadcasts block fragments.
type NewBlockFragmentEvent struct {
	BlockNumber *big.Int
	Round       uint64
	Data        *types.BlockFragment
}

// NewMajorityEvent is posted when there's a majority during a sub election
type NewMajorityEvent struct {
	Winner common.Hash
}
