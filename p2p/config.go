package p2p

import (
	"time"

	"github.com/libp2p/go-libp2p-crypto"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p/config"
)

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
