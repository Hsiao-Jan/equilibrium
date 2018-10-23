package types

import (
	"math/big"
	"reflect"

	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
)

var _ Evidence = (*DuplicateVoting)(nil)

// Evidence is the information that is used to decide the case.
type Evidence interface {
	Hash() Hash
	FactFinder() (Address, error)
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
func (c *Conviction) Category() string {
	return reflect.TypeOf(c.evidence).Name()
}

// Convictions is a Conviction slice type for basic sorting.
type Convictions []*Conviction

// Len returns the length of s.
func (c Convictions) Len() int { return len(c) }

// Swap swaps the i'th and the j'th element in s.
func (c Convictions) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (c Convictions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(c[i])
	return enc
}

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
