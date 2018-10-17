package p2p

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/kowala-tech/client-experimental/log"
	libp2p_host "github.com/libp2p/go-libp2p-host"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

func bootstrapConnect(ctx context.Context, ph libp2p_host.Host, peers []pstore.PeerInfo) error {
	if len(peers) == 0 {
		return errors.New("not enough bootstrap peers")
	}

	log.Logger.Info("Connecting to bootstrap nodes ...")

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
			defer log.Logger.Debug("bootstrapDial", "from", ph.ID().Pretty(), "bootstrapping to", p.ID)
			log.Logger("bootstrapDial", "from", ph.ID(), "bootstrapping to", p.ID)

			ph.Peerstore().AddAddrs(p.ID, p.Addrs, pstore.PermanentAddrTTL)
			if err := ph.Connect(ctx, p); err != nil {
				log.Error("Failed to bootstrap with", "ID", p.ID, "err", err)
				errs <- err
				return
			}
			log.Logger.Debug("bootstrapDialSuccess", "ID", p.ID)
			log.Logger.Info("Bootstrapped with", "ID", p.ID)
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
