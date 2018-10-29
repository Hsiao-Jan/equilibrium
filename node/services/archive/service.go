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

package archive

import (
	"github.com/kowala-tech/equilibrium/node"
	"github.com/kowala-tech/equilibrium/node/p2p"
	"github.com/kowala-tech/equilibrium/node/services/archive/genesis"
	"github.com/kowala-tech/equilibrium/node/services/archive/types/chain"
)

const (
	chainDataDirName = "chaindata"
)

// Service represents the archive service.
type Service struct {
	blockChain chain.BlockChain
}

// New retrieves a new instance of the archive service.
func New(config *Config, ctx *node.Context) (*Service, error) {
	// @TODO (RGERALDES) add resolve path for ctx (chainDataDirName)
	chainDB, err := OpenDB(ctx.DataDir(), chainDataDirName, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}

	if err := genesis.Setup(chainDB, config.Genesis); err != nil {
		return nil, err
	}

	cacheConfig := &chain.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}}
	blockChain, err := chain.New(chainDB, cache)
	if err != nil {
		return nil, err
	}

	return &Service{}, nil
}

func (arc *Service) Start(server *p2p.Host) error { return nil }
func (arc *Service) Stop() error                  { return nil }
