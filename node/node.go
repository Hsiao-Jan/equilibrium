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

package node

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"

	"github.com/kowala-tech/equilibrium/p2p"
	"github.com/kowala-tech/kcoin/client/event"
	"github.com/prometheus/prometheus/util/flock"
)

var (
	errNodeRunning = errors.New("node already running")
	errDatadirUsed = errors.New("datadir already used by another process")

	datadirInUseErrnos = map[uint]bool{11: true, 32: true, 35: true}
)

// Node is a container for blockchain services.
type Node struct {
	nodeMu sync.RWMutex
	cfg    Config

	hostCfg p2p.Config
	host    *p2p.Host

	serviceFuncs []ServiceConstructor     // Service constructors (in dependency order)
	services     map[reflect.Type]Service // Currently running services

	globalEventMux *event.TypeMux

	dirLock flock.Releaser // prevents concurrent use of instance directory

	doneCh chan struct{}
}

// Start initiates the node operations/services.
func (n *Node) Start() error {
	n.nodeMu.Lock()
	defer n.nodeMu.Unlock()

	if n.IsRunning() {
		return errNodeRunning
	}

	if err := n.openDataDir(); err != nil {
		return err
	}

}

// IsRunning reports whether the node is running or not.
func (n *Node) IsRunning() bool {
	return n.host != nil
}

func (n *Node) openDataDir() error {
	if n.cfg.DataDir == "" {
		return nil // ephemeral
	}

	dir := filepath.Join(n.cfg.DataDir, n.cfg.name())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Lock the instance directory to prevent concurrent use by another instance as well as
	// accidental use of the instance directory as a database.
	release, _, err := flock.New(filepath.Join(dir, "LOCK"))
	if err != nil {
		return convertFileLockError(err)
	}
	n.dirLock = release
	return nil
}

func convertFileLockError(err error) error {
	if errno, ok := err.(syscall.Errno); ok && datadirInUseErrnos[uint(errno)] {
		return errDatadirUsed
	}
	return err
}
