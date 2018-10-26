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

package types

import (
	"math/big"
	"reflect"

	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/state/accounts"
)

var _ Evidence = (*DuplicateVoting)(nil)

// Evidence is the information that is used to decide the case.
type Evidence interface {
	Hash() crypto.Hash
	FactFinder(signer crypto.Signer) (accounts.Address, error)
}

// Conviction is the verdict that results when a validator
// finds other validator guilty of protocol violation.
type Conviction struct {
	blockNumber *big.Int         `json:"blockNumber" gencodec:"required"`
	perpetrator accounts.Address `json:"perpetrator" gencodec:"required"`
	evidence    Evidence         `json:"evidence"    gencodec:"required"`
}

// convictionMarshaling field type overrides for gencodec.
type convictionMarshaling struct {
	Number *hexutil.Big
}

// Category returns the conviction category.
func (c *Conviction) Category() string { return reflect.TypeOf(c.evidence).Name() }

// Perpetrator returns the conviction perpetrator.
func (c *Conviction) Perpetrator() accounts.Address { return c.perpetrator }

// Evidence returns the conviction evidence.
func (c *Conviction) Evidence() Evidence { return c.evidence }

// DuplicateVoting satisfies the Evidence interface.
type DuplicateVoting struct {
	Vote1 *Vote `json:"vote1" gencodec:"required"`
	Vote2 *Vote `json:"vote2" gencodec:"required"`

	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

// Hash returns DuplicateVoting hash.
func (dv *DuplicateVoting) Hash() crypto.Hash {
	return crypto.RLPHash(dv)
}

// FactFinder returns the validator responsible for gathering the facts.
func (dv *DuplicateVoting) FactFinder(signer crypto.Signer) (accounts.Address, error) {
	return accounts.Address{}, nil
}
