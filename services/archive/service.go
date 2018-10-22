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
	"github.com/kowala-tech/equilibrium/types"
	"github.com/kowala-tech/kcoin/client/core"
)

// @TODO - consensus engine should be part of the service context

type Service struct {
	cfg *Config

	// handlers
	blockchain *types.Blockchain
	engine     consensus.engine
	//txPool      *types.TxPool
	//peerManager *PeerManager

}

// New retrieves a new instance of the archive service.
func New(cfg *Config, ctx *node.ServiceContext) (*Service, error) {
	service := &Service{
		engine: ctx.ConsensusEngine,
	}

	/*
		if !config.SyncMode.IsValid() {
			return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
		}
	*/

	kcoin.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, kcoin.chainConfig, service.engine, vmConfig)
	if err != nil {
		return nil, err
	}

	return service, nil
}
