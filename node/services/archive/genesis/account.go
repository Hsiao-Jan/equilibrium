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

package genesis

import (
	"math/big"

	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/state/accounts"
)

// Accounts specifies the initial state that is part of the genesis block.
type Accounts map[accounts.Address]Account

// Account represents a genesis account.
type Account struct {
	Code    []byte                      `json:"code,omitempty"`
	Storage map[crypto.Hash]crypto.Hash `json:"storage,omitempty"`
	Balance *big.Int                    `json:"balance,omitempty"`
}
