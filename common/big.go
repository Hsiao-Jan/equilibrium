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

package common

import (
	"math/big"
)

var (
	Big0   = new(big.Int).SetUint64(0)
	Big1   = new(big.Int).SetUint64(1)
	Big8   = new(big.Int).SetUint64(8)
	Big32  = new(big.Int).SetUint64(32)
	Big100 = new(big.Int).SetUint64(100)
)

// Min returns the smallest big int
func Min(b1 *big.Int, b2 *big.Int) *big.Int {
	if b1.Cmp(b2) <= 0 {
		return b1
	}
	return b2
}
