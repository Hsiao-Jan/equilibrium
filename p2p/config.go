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

package p2p

import (
	"time"

	"github.com/libp2p/go-libp2p-crypto"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p/config"
)

// DefaultConfig holds the default config values for the p2p package.
var DefaultConfig = Config{
	IsBootstrappingNode: false,
	MinPeers:            15,
	MaxPeers:            25,
	PruningGracePeriod:  30 * time.Second,
}

// Config holds the p2p host options.
type Config struct {
	PrivateKey *crypto.PrivKey

	BootstrappingNodes []pstore.PeerInfo

	IsBootstrappingNode bool

	ListenAddr string

	NATMgr config.NATManagerC

	MaxPeers int

	MinPeers int

	PruningGracePeriod time.Duration
}
