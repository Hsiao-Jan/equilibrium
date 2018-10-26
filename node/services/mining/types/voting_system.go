package types

import (
	"errors"
	"math/big"

	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/node/event"
)

// VotingTables represents the voting tables available for each election round
type VotingTables = [2]VotingTable

func NewVotingTables(eventMux *event.TypeMux, voters Voters) (VotingTables, error) {
	majorityFunc := func(winnerBlock crypto.Hash) {
		go eventMux.Post(NewMajorityEvent{Winner: winnerBlock})
	}

	var err error
	tables := VotingTables{}

	// prevote
	tables[PreVote], err = NewVotingTable(PreVote, voters, majorityFunc)
	if err != nil {
		return tables, err
	}

	// precommit
	tables[PreCommit], err = NewVotingTable(PreCommit, voters, majorityFunc)
	if err != nil {
		return tables, err
	}

	return tables, nil
}

// NewVoteEvent is posted when the voting system processes a vote successfully.
type NewVoteEvent struct{ Vote *Vote }

// VotingSystem records the election votes since round 1
type VotingSystem struct {
	voters         Voters
	electionNumber *big.Int // election number
	round          uint64
	votesPerRound  map[uint64]VotingTables

	eventMux *event.TypeMux
}

// NewVotingSystem returns a new voting system
func NewVotingSystem(eventMux *event.TypeMux, electionNumber *big.Int, voters Voters) (*VotingSystem, error) {
	system := &VotingSystem{
		voters:         voters,
		electionNumber: electionNumber,
		round:          0,
		votesPerRound:  make(map[uint64]VotingTables),
		eventMux:       eventMux,
	}

	err := system.NewRound()
	if err != nil {
		return nil, err
	}

	return system, nil
}

func (vs *VotingSystem) NewRound() error {
	var err error
	vs.votesPerRound[vs.round], err = NewVotingTables(vs.eventMux, vs.voters)
	if err != nil {
		return err
	}
	return nil
}

// Add registers a vote
func (vs *VotingSystem) Add(vote AddressVote) error {
	votingTable, err := vs.getVoteSet(vote.Vote().Round(), vote.Vote().Type())
	if err != nil {
		return err
	}

	err = votingTable.Add(vote)
	if err != nil {
		return err
	}

	go vs.eventMux.Post(NewVoteEvent{Vote: vote.Vote()})

	return nil
}

func (vs *VotingSystem) Leader(round uint64, voteType VoteType) (crypto.Hash, error) {
	votingTable, err := vs.getVoteSet(round, voteType)
	if err != nil {
		return crypto.Hash{}, err
	}

	return votingTable.Leader(), nil
}

func (vs *VotingSystem) getVoteSet(round uint64, voteType VoteType) (VotingTable, error) {
	votingTables, ok := vs.votesPerRound[round]
	if !ok {
		return nil, errors.New("voting table for round doesnt exists")
	}

	if uint64(voteType) > uint64(len(votingTables)-1) {
		return nil, errors.New("invalid voteType on add vote ")
	}

	return votingTables[int(voteType)], nil
}
