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

package mining

import (
	"math/big"

	"github.com/kowala-tech/equilibrium/types"
	"github.com/kowala-tech/kcoin/client/common/hexutil"
)

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

var DefaultConfig = Config{}

type Config struct {
	Coinbase  types.Address `toml:",omitempty"`
	Deposit   *big.Int      `toml:",omitempty"`
	ExtraData []byte        `toml:",omitempty"`
}

type configMarshaling struct {
	ExtraData hexutil.Bytes
}
