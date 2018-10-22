package validator

import (
	"math/big"
	"time"

	"github.com/kowala-tech/kcoin/client/core/types"
	"github.com/kowala-tech/kcoin/client/event"
)

// VotingState encapsulates the consensus state for a specific block election
type VotingState struct {
	blockNumber *big.Int
	round       uint64

	voters         types.Voters
	votersChecksum [32]byte

	proposal       *types.Proposal
	block          *types.Block
	blockFragments *types.BlockFragments
	votingSystem   *VotingSystem // election votes since round 1

	lockedRound uint64
	lockedBlock *types.Block

	start time.Time // used to sync the validator nodes

	commitRound int

	// inputs
	blockCh  chan *types.Block
	majority *event.TypeMuxSubscription

	// state changes related to the election
	*work
}
