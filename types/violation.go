package types

import (
	"math/big"
	"reflect"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
)

var _ Evidence = (*DuplicateVoting)(nil)

// Evidence is the information that is used to decide the case.
type Evidence interface {
	Hash() Hash
	FactFinder(signer Signer) (Address, error)
}

// Conviction is the verdict that results when a validator
// finds other validator guilty of protocol violation.
type Conviction struct {
	blockNumber *big.Int `json:"blockNumber" gencodec:"required"`
	perpetrator Address  `json:"perpetrator" gencodec:"required"`
	evidence    Evidence `json:"evidence"    gencodec:"required"`
}

// convictionMarshaling field type overrides for gencodec.
type convictionMarshaling struct {
	Number *hexutil.Big
}

// Category returns the conviction category.
func (c *Conviction) Category() string { return reflect.TypeOf(c.evidence).Name() }

// Perpetrator returns the conviction perpetrator.
func (c *Conviction) Perpetrator() string { return c.perpetrator }

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
func (dv *DuplicateVoting) Hash() Hash {
	return rlpHash(dv)
}

// FactFinder returns the validator responsible for gathering the facts.
func (dv *DuplicateVoting) FactFinder(signer Signer) (Address, error) {
	addr, err := signer.Sender(dv)
	if err != nil {
		return common.Address{}, err
	}
	return addr, nil
}
