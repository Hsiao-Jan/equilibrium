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
	"github.com/kowala-tech/equilibrium/node/services/archive/genesis"
)

// Config contains the archive service configurations.
type Config struct {
	Genesis *genesis.Genesis

	// Database options
	DatabaseHandles int
	DatabaseCache   int
}

/*
//go:generate gencodec -type Config -formats toml -out gen_config.go

// DefaultConfig contains default settings for use on the Kowala main net.
var DefaultConfig = Config{
	SyncMode:      downloader.FastSync,
	DatabaseCache: 128,
	TrieCache:     256,
	TrieTimeout:   60 * time.Minute,

	//TxPool: core.DefaultTxPoolConfig,
}

type Config struct {
	Genesis *genesis.Genesis


	SyncMode downloader.SyncMode

	// The genesis block, which is inserted if the database is empty.
	// If nil, the Ethereum main net block is used.
	//Genesis *core.Genesis `toml:",omitempty"`

	// Protocol options
	//NetworkId uint64 // Network ID to use for selecting peers to connect to

	NoPruning bool

	// Database options
	SkipBcVersionCheck bool `toml:"-"`
	DatabaseHandles    int  `toml:"-"`
	DatabaseCache      int
	TrieCache          int
	TrieTimeout        time.Duration

	// Transaction pool options
	//TxPool core.TxPoolConfig

	// Enables tracking of SHA3 preimages in the VM
	EnablePreimageRecording bool

	// Miscellaneous options
	DocRoot string `toml:"-"`

}
*/
