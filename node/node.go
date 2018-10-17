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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"

	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/p2p"
	"github.com/kowala-tech/kcoin/client/event"
	"github.com/prometheus/prometheus/util/flock"
	"go.uber.org/zap"
)

const (
	datadirDefaultKeyStore = "keystore" // Path within the datadir to the keystore
)

var (
	errNodeRunning = errors.New("node already running")
	errDatadirUsed = errors.New("datadir already used by another process")
	errNodeStopped = errors.New("node not started")

	datadirInUseErrnos = map[uint]bool{11: true, 32: true, 35: true}
)

// duplicateServiceError is returned during Node startup if a registered service
// constructor returns a service of the same type that was already started.
type duplicateServiceError struct {
	Kind reflect.Type
}

// Error generates a textual representation of the duplicate service error.
func (e *duplicateServiceError) Error() string {
	return fmt.Sprintf("duplicate service: %v", e.Kind)
}

// stopError is returned if a Node fails to stop either any of its registered
// services or itself.
type stopError struct {
	Server   error
	Services map[reflect.Type]error
}

// Error generates a textual representation of the stop error.
func (e *stopError) Error() string {
	return fmt.Sprintf("server: %v, services: %v", e.Server, e.Services)
}

// Node is a container for blockchain services.
type Node struct {
	nodeMu sync.RWMutex
	cfg    *Config

	hostCfg p2p.Config
	host    *p2p.Host

	serviceFuncs []ServiceConstructor     // Service constructors (in dependency order)
	services     map[reflect.Type]Service // Currently running services

	globalEventMux *event.TypeMux

	dirLock flock.Releaser // prevents concurrent use of instance directory

	doneCh chan struct{}
}

// New create a new Kowala node, ready for service registration.
func New(ctx context.Context, cfg *Config) (*Node, error) {
	// Copy config and resolve the datadir so future changes to the current
	// working directory don't affect the node.
	cfgCopy := *cfg
	cfg = &cfgCopy
	if cfg.DataDir != "" {
		absdatadir, err := filepath.Abs(cfg.DataDir)
		if err != nil {
			return nil, err
		}
		cfg.DataDir = absdatadir
	}
	// Ensure that the instance name doesn't cause weird conflicts with
	// other files in the data directory.
	if strings.ContainsAny(cfg.Name, `/\`) {
		return nil, errors.New(`Config.Name must not contain '/' or '\'`)
	}
	if cfg.Name == datadirDefaultKeyStore {
		return nil, errors.New(`Config.Name cannot be "` + datadirDefaultKeyStore + `"`)
	}
	if strings.HasSuffix(cfg.Name, ".ipc") {
		return nil, errors.New(`Config.Name cannot end in ".ipc"`)
	}

	return &Node{
		cfg:            cfg,
		serviceFuncs:   []ServiceConstructor{},
		globalEventMux: new(event.TypeMux),
	}, nil
}

// Register injects a new service into the node's stack. The service created by
// the passed constructor must be unique in its type with regard to sibling ones.
func (n *Node) Register(constructor ServiceConstructor) error {
	n.nodeMu.Lock()
	defer n.nodeMu.Unlock()

	if !n.IsRunning() {
		return errNodeRunning
	}
	n.serviceFuncs = append(n.serviceFuncs, constructor)
	return nil
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

	n.hostCfg = n.cfg.P2P
	//n.hostCfg.PrivateKey = n.config.NodeKey()
	//n.hostCfg.Name = n.cfg.NodeName()

	host := p2p.NewHost(n.hostCfg)
	//log.Info("Starting p2p node", zap.String("instance", n.hostCfg.Name))

	// Otherwise copy and specialize the P2P configuration
	services := make(map[reflect.Type]Service)
	for _, constructor := range n.serviceFuncs {
		// Create a new context for the particular service
		ctx := &ServiceContext{
			cfg:            n.cfg,
			services:       make(map[reflect.Type]Service),
			GlobalEventMux: n.globalEventMux,
		}
		for kind, s := range services { // copy needed for threaded access
			ctx.services[kind] = s
		}
		// Construct and save the service
		service, err := constructor(ctx)
		if err != nil {
			return err
		}
		kind := reflect.TypeOf(service)
		if _, exists := services[kind]; exists {
			return &duplicateServiceError{Kind: kind}
		}
		services[kind] = service
	}

	if err := host.Start(); err != nil {
		return convertFileLockError(err)
	}

	// Start each of the services
	started := []reflect.Type{}
	for kind, service := range services {
		// Start the next service, stopping all previous upon failure
		if err := service.Start(host); err != nil {
			for _, kind := range started {
				services[kind].Stop()
			}
			host.Stop()
			return err
		}
		// Mark the service started for potential cleanup
		started = append(started, kind)
	}

	// Finish initializing the startup
	n.services = services
	n.host = host
	n.doneCh = make(chan struct{})

	log.Info("Node Started")

	return nil
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

// Stop terminates a running node along with all it's services. In the node was
// not started, an error is returned.
func (n *Node) Stop() error {
	n.nodeMu.Lock()
	defer n.nodeMu.Unlock()

	// Short circuit if the node's not running
	if !n.IsRunning() {
		return errNodeStopped
	}

	// Terminate the API, services and the p2p server.
	failure := &stopError{
		Services: make(map[reflect.Type]error),
	}
	for kind, service := range n.services {
		if err := service.Stop(); err != nil {
			failure.Services[kind] = err
		}
	}
	n.host.Stop()
	n.services = nil
	n.host = nil

	// Release instance directory lock.
	if n.dirLock != nil {
		if err := n.dirLock.Release(); err != nil {
			log.Error("Can't release datadir lock", zap.Error(err))
		}
		n.dirLock = nil
	}

	// unblock n.Wait
	close(n.doneCh)

	if len(failure.Services) > 0 {
		return failure
	}
	return nil
}

// Wait blocks the thread until the node is stopped. If the node is not running
// at the time of invocation, the method immediately returns.
func (n *Node) Wait() {
	n.nodeMu.RLock()
	if !n.IsRunning() {
		n.nodeMu.RUnlock()
		return
	}
	stop := n.doneCh
	n.nodeMu.RUnlock()

	<-stop
}
