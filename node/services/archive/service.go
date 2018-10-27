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
	"github.com/kowala-tech/equilibrium/node"
	"github.com/kowala-tech/equilibrium/node/services/archive/types/chain"
)

// Service represents the archive service.
type Service struct {
	cfg *Config

	blockchain *chain.Blockchain
}

// New retrieves a new instance of the archive service.
func New(cfg *Config, ctx *node.ServiceContext) (*Service, error) {
	return &Service{}, nil
}
