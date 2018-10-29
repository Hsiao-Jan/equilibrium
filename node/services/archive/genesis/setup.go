// Copyright © 2018 Kowala SEZC <info@kowala.tech>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genesis

import (
	"errors"
	"math/big"

	"github.com/kowala-tech/equilibrium/state/accounts"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/database"
	"github.com/kowala-tech/equilibrium/database/rawdb"
	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
	"github.com/kowala-tech/equilibrium/state"
)

var (
	// ErrGenesisUnavailable is thrown if the genesis has not been provided
	// given an empty chain database. Kowala has decided to not use a default
	// genesis since it's an critical step and thus force the user to acknowledge
	// the problem.
	ErrGenesisUnavailable = errors.New("genesis not available")
)

// Genesis defines the initial condittions for a Kowala blockchain.
type Genesis struct {
	ComputeCapacity  *big.Int `json:"computeCapacity" gencodec:"required"`
	ComputeUnitPrice *big.Int `json:"computeUnitCost" gencodec:"required"`
	Timestamp        uint64   `json:"timestamp"       gencodec:"required"`
	ExtraData        []byte   `json:"extraData"`
	Accounts         Accounts `json:"accounts"        gencodec:"required"`
}

// ToBlock converts the genesis into a chain block.
func (gen *Genesis) ToBlock(db database.Database) (*types.Block, error) {
	if db == nil {
		db = database.NewMemDatabase()
	}

	stateDB, _ := state.New(crypto.Hash{}, state.NewDatabase(db))
	for addr, account := range gen.Accounts {
		if account.Balance == nil {
			account.Balance = common.Big0
		}
		stateDB.AddBalance(addr, account.Balance)
		stateDB.SetCode(addr, account.Code)
		for key, value := range account.Storage {
			stateDB.SetState(addr, key, value)
		}
	}

	root := stateDB.IntermediateRoot(false)

	head := &types.Header{
		Number:            common.Big0,
		Time:              new(big.Int).SetUint64(gen.Timestamp),
		PreviousBlockHash: crypto.Hash{},
		Extra:             gen.ExtraData,
		Proposer:          accounts.Address{},
		Snapshot:          root,
	}

	stateDB.Commit(false)
	stateDB.Database().TrieDB().Commit(root, true)

	return types.NewBlock(head, nil, nil, nil, nil)
}

// Setup writes the genesis block and returns the network settings.
func Setup(db database.Database, gen *Genesis) error {
	stored := rawdb.ReadCanonicalHash(db, 0)
	if (stored == crypto.Hash{}) {
		// Force the user to acknowledge the genesis instead of using a
		// default genesis since it's a vital operation.
		if gen == nil {
			return ErrGenesisUnavailable
		}

		log.Info("Writing genesis block...")
		block, err := gen.ToBlock(db)
		if err != nil {
			return err
		}
		commit(db, block)

		return nil
	}

	if gen != nil {
		log.Info("Failed to use new genesis; db not empty")
	}

	return nil
}

func commit(db database.Database, block *types.Block) {
	rawdb.WriteBlock(db, block)
	rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), nil)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteHeadHeaderHash(db, block.Hash())
}
