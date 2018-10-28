package genesis

import (
	"errors"

	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/state"
	"github.com/kowala-tech/equilibrium/database"
	"github.com/kowala-tech/equilibrium/database/rawdb"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/network"
)

var (
	ErrGenesisUnavailable = errors.New("genesis is not available")
)

// Genesis defines the initial condittions for a blockchain.
type Genesis struct {
	Network   *network.Settings `json:"network"   gencodec:"required"`
	Timestamp uint64			`json:"timestamp" gencodec:"required"`
	ExtraData string            `json:"extra"`
	Accounts Accounts			`json:"accounts"  gencodec:"required"`
}

func (gen *Genesis) ToBlock(db database.Database) *types.Block{
	if db == nil {
		db = database.NewMemDatabase()
	}

	statedb, _ := state.New(crypto.Hash{}, state.NewDatabase(db))
	for addr, account := range g.Alloc {
		statedb.AddBalance(addr, account.Balance)
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}
	root := statedb.IntermediateRoot(false)
	head := &types.Header{
		Number:     new(big.Int).SetUint64(g.Number),
		Time:       new(big.Int).SetUint64(g.Timestamp),
		ParentHash: g.ParentHash,
		Extra:      g.ExtraData,
		GasLimit:   g.GasLimit,
		GasUsed:    g.GasUsed,
		Coinbase:   g.Coinbase,
		Root:       root,
	}
	if g.GasLimit == 0 {
		head.GasLimit = params.GenesisGasLimit
	}
	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(root, true)

	return types.NewBlock(head, nil, nil, nil)
}

// Setup writes the genesis block and returns the network settings.
func Setup(db database.Database, newGenesis *Genesis) (*network.Settings, error) {
	stored := rawdb.ReadCanonicalHash(db, 0) 
	if (stored == crypto.Hash{}) {
		// Force the user to acknowledge the genesis instead of using a
		// default genesis since it's a vital operation.
		if newGenesis == nil {
			return nil, ErrGenesisUnavailable
		}

		log.Info("Writing block")
		genesisBlock := newGenesis.ToBlock()
		commit(db, genesisBlock)

		log.Info("Writing network settings")
		rawdb.WriteNetworkSettings(db, block.Hash(), newGenesis.Network)

		return newGenesis.Config, err
	}

	if newGenesis != nil {
		log.Info("provided genesis ignored - db not empty")
	}

	// Get the existing network settings.
	return rawdb.ReadNetworkSettings(db, stored), nil
}

func commit(db database.Database, block *types.Block) {
	rawdb.WriteBlock(db, block)
	rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), nil)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteHeadHeaderHash(db, block.Hash())
}