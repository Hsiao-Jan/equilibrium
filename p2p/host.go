package p2p

import (
	"context"
	"errors"

	dstore "github.com/ipfs/go-datastore"
	ipfssync "github.com/ipfs/go-datastore/sync"
	"github.com/kowala-tech/equilibrium/log"
	pubsub "github.com/libp2p/go-floodsub"
	libp2p "github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	libp2p_host "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"go.uber.org/zap"
)

var (
	errNoPrivateKey = errors.New("Host.PrivateKey must be set to a non-nil key")
)

// Host represents a p2p host.
type Host struct {
	Config
	libp2p_host.Host
	*pubsub.PubSub
}

// NewHost returns a new p2p host.
func NewHost(cfg Config) *Host {
	return &Host{Config: cfg}
}

// Start initiates the host operations.
func (h *Host) Start() error {
	if h.PrivateKey == nil {
		return errNoPrivateKey
	}

	connMgr := connmgr.NewConnManager(h.MinPeers, h.MaxPeers, h.PruningGracePeriod)

	ctx := context.Background()
	host, err := libp2p.New(
		ctx,
		libp2p.Identity(*h.PrivateKey),
		libp2p.ListenAddrStrings(h.ListenAddr),
		libp2p.NATPortMap(),
		libp2p.ConnectionManager(connMgr),
	)
	if err != nil {
		return err
	}
	h.Host = host

	pubsub, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		return err
	}
	h.PubSub = pubsub

	dht := dht.NewDHT(ctx, host, ipfssync.MutexWrap(dstore.NewMapDatastore()))

	if len(h.BootstrappingNodes) > 0 {
		routedHost := rhost.Wrap(host, dht)
		if err := bootstrapConnect(ctx, routedHost, h.BootstrappingNodes); err != nil {
			log.Error("Could not connect to the bootstrap nodes", zap.Error(err))
		}
	}

	if h.IsBootstrappingNode {
		if err := dht.Bootstrap(ctx); err != nil {
			return err
		}
	}

	log.Info("Listening...", zap.String("ID", host.ID().Pretty()), zap.String("addr", h.ListenAddr))

	return nil
}

// Stop terminates the host operations.
func (h *Host) Stop() error {
	return h.Host.Close()
}