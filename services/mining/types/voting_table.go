package types

import (
	"errors"
	"fmt"

	"github.com/kowala-tech/equilibrium/accounts"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/types"
	"github.com/kowala-tech/kcoin/client/common"
)

// NewMajorityEvent is posted by a voting table when there's a majority during an election.
type NewMajorityEvent struct {
	Winner common.Hash
}

// VotingTable represents a consensus sub-election voting table.
type VotingTable interface {
	Add(vote AddressVote) error
	Leader() crypto.Hash
}

type votingTable struct {
	voteType VoteType
	voters   Voters
	votes    *VotesSet
	quorum   QuorumFunc
	majority QuorumReachedFunc
}

func NewVotingTable(voteType VoteType, voters Voters, majority QuorumReachedFunc) (*votingTable, error) {
	if voters == nil {
		return nil, errors.New("cant create a voting table with nil voters")
	}

	return &votingTable{
		voteType: voteType,
		voters:   voters,
		votes:    types.NewVotesSet(),
		quorum:   TwoThirdsPlusOneVoteQuorum,
		majority: majority,
	}, nil
}

func (table *votingTable) Add(voteAddressed AddressVote) error {
	if !table.isVoter(voteAddressed.Address()) {
		return fmt.Errorf("voter address not found in voting table: 0x%x", voteAddressed.Address().Hash())
	}
	if err := table.isDuplicate(voteAddressed); err != nil {
		return err
	}

	vote := voteAddressed.Vote()
	table.votes.Add(vote)

	if table.hasQuorum() {
		log.Debug("voting. Quorum has been achieved. majority", "votes", table.votes.Len(), "voters", table.voters.Len())
		table.majority(vote.BlockHash())
	}

	return nil
}

func (table *votingTable) Leader() crypto.Hash {
	return table.votes.Leader()
}

func (table *votingTable) isDuplicate(voteAddressed AddressVote) error {
	vote := voteAddressed.Vote()
	err := table.votes.Contains(vote.Hash())
	if err != nil {
		log.Error(fmt.Sprintf("a duplicate vote in voting table %v; blockHash %v; voteHash %v; from validator %v. Error: %s",
			table.voteType, vote.BlockHash(), vote.Hash(), voteAddressed.Address(), vote.String()))
	}
	return err
}

func (table *votingTable) isVoter(address accounts.Address) bool {
	return table.voters.Contains(address)
}

func (table *votingTable) hasQuorum() bool {
	log.Debug("voting. hasQuorum", "voters", table.voters.Len(), "votes", table.votes.Len())
	return table.quorum(table.votes.Len(), table.voters.Len())
}

type QuorumReachedFunc func(winner crypto.Hash)

type QuorumFunc func(votes, voters int) bool

func TwoThirdsPlusOneVoteQuorum(votes, voters int) bool {
	return votes >= voters*2/3+1
}
