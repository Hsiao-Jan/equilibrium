package node

import "github.com/kowala-tech/equilibrium/p2p"

var DefaultConfig = Config{}

// Config represents
type Config struct {
	P2P p2p.Config
}
