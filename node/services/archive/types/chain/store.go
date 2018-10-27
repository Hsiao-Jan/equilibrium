package types

import (
	"crypto"

	"github.com/kowala-tech/equilibrium/encoding/rlp"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
)

const (
	bodyCacheLimit   = 256
	blockCacheLimit  = 256
	maxFutureBlocks  = 256
	headerCacheLimit = 512
	numberCacheLimit = 2048
)

type BlockStore interface {
	HeaderStore
	GetBody(hash crypto.Hash) *Body
	GetBodyRLP(hash crypto.Hash) rlp.RawValue
	HasBlock(hash crypto.Hash, number uint64) bool
	GetBlock(hash crypto.Hash, number uint64) *Block
	PurgeBlockCaches()
}

type HeaderStore interface {
	HasHeader(has crypto.Hash, number uint64) bool
	GetHeader(hash crypto.Hash, number uint64) *types.Header
	GetBlockNumber(hash crypto.Hash) *uint64
	PurgeHeaderCaches()
	AddHeader(hash crypto.Hash, header *types.Header)
	AddNumber(hash crypto.Hash, number uint64)
}
