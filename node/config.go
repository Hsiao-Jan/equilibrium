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
	"os"
	"path/filepath"
	"strings"

	"github.com/kowala-tech/equilibrium/node/p2p"
)

var DefaultConfig = Config{
	P2P: p2p.DefaultConfig,
}

// Config holds the node configurations.
type Config struct {
	Name string

	DataDir string

	P2P p2p.Config
}

func (c *Config) name() string {
	if c.Name == "" {
		progname := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
		if progname == "" {
			panic("empty executable name, set Config.Name")
		}
		return progname
	}
	return c.Name
}
