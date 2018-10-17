package p2p

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/kowala-tech/equilibrium/log"
	libp2p_host "github.com/libp2p/go-libp2p-host"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

func bootstrapConnect(ctx context.Context, ph libp2p_host.Host, peers []pstore.PeerInfo) error {
	if len(peers) == 0 {
		return errors.New("not enough bootstrap peers")
	}

	log.Info("Connecting to bootstrap nodes ...")

	errs := make(chan error, len(peers))
	var wg sync.WaitGroup
	for _, p := range peers {

		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.
		// Also, performed asynchronously for dial speed.

		wg.Add(1)
		go func(p pstore.PeerInfo) {
			defer wg.Done()
			defer log.Debug("bootstrapDial", zap.String("from", ph.ID().Pretty()), zap.String("bootstrapping to", p.ID.Pretty()))
			log.Info("bootstrapDial", zap.String("from", ph.ID().Pretty()), zap.String("bootstrapping to", p.ID.Pretty()))

			ph.Peerstore().AddAddrs(p.ID, p.Addrs, pstore.PermanentAddrTTL)
			if err := ph.Connect(ctx, p); err != nil {
				log.Error("Failed to bootstrap with", zap.String("ID", p.ID.Pretty()), zap.Error(err))
				errs <- err
				return
			}
			log.Debug("bootstrapDialSuccess", zap.String("ID", p.ID.Pretty()))
			log.Info("Bootstrapped with", zap.String("ID", p.ID.Pretty()))
		}(p)
	}
	wg.Wait()

	// our failure condition is when no connection attempt succeeded.
	// So drain the errs channel, counting the results.
	close(errs)
	count := 0
	var err error
	for err = range errs {
		if err != nil {
			count++
		}
	}
	if count == len(peers) {
		return fmt.Errorf("Failed to bootstrap. %s", err)
	}
	return nil
}
