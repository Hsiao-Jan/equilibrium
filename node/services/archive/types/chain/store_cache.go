package chain

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
)

var _ BlockStore = (*Cache)(nil)

type Cache struct {
	BlockStore

	// block related caches
	bodyCache    *lru.Cache // Cache for the most recent block bodies
	bodyRLPCache *lru.Cache // Cache for the most recent block bodies in RLP encoded format
	blockCache   *lru.Cache // Cache for the most recent entire blocks
	futureBlocks *lru.Cache // future blocks are blocks added for later processing

	// header related caches
	headerCache *lru.Cache // Cache for the most recent block headers
	numberCache *lru.Cache // Cache for the most recent block numbers
}

func NewCache(store BlockStore) *Cache {
	bodyCache, _ := lru.New(bodyCacheLimit)
	bodyRLPCache, _ := lru.New(bodyCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)
	headerCache, _ := lru.New(headerCacheLimit)
	numberCache, _ := lru.New(numberCacheLimit)

	return &Cache{
		Store:        store,
		bodyCache:    bodyCache,
		bodyRLPCache: bodyRLPCache,
		blockCache:   blockCache,
		futureBlocks: futureBlocks,
		headerCache:  headerCache,
		numberCache:  numberCache,
	}
}

func (cc *Cache) HasHeader(hash crypto.Hash, number uint64) bool {
	if cc.numberCache.Contains(hash) || cc.headerCache.Contains(hash) {
		return true
	}
	return cc.Store.HasHeader(hc.chainDb, hash, number)
}

func (cc *Cache) GetHeader(hash crypto.Hash, number uint64) *types.Header {
	if header, ok := cc.headerCache.Get(hash); ok {
		return header.(*Header)
	}

	header := cc.Store.GetHeader(hash, number)
	if header == nil {
		return nil
	}

	hc.headerCache.Add(hash, header)

	return header
}

func (cc *Cache) GetBlockNumber(hash crypto.Hash) *uint64 {
	if cached, ok := cc.numberCache.Get(hash); ok {
		number := cached.(uint64)
		return &number
	}

	number := cc.Store.GetBlockNumber(hash)
	if number != nil {
		cc.numberCache.Add(hash, *number)
	}

	return number
}

func (cc *Cache) GetBlock(hash crypto.Hash, number uint64) *types.Block {
	if block, ok := cc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}

	block := cc.Store.GetBlock(hash, number)
	if block == nil {
		return nil
	}

	cc.blockCache.Add(block.Hash(), block)

	return block
}

func (cc *Cache) HasBlock(hash crypto.Hash, number uint64) bool {
	if cc.blockCache.Contains(hash) {
		return true
	}

	return cc.Store.HasBlock(hash, number)
}

func (cc *Cache) GetBody(hash crypto.Hash) *types.Body {
	if cached, ok := cc.bodyCache.Get(hash); ok {
		body := cached.(*types.Body)
		return body
	}

	body := cc.Store.GetBody(hash)
	if body == nil {
		return nil
	}

	cc.bodyCache.Add(hash, body)

	return body
}

func (cc *Cache) GetBodyRLP(hash crypto.Hash) rlp.RawValue {
	if cached, ok := cc.bodyRLPCache.Get(hash); ok {
		return cached.(rlp.RawValue)
	}

	body := cc.Store.GetBodyRLP(hash)
	if len(body) == 0 {
		return nil
	}

	cc.bodyRLPCache.Add(hash, body)

	return body
}

func (cc *Cache) PurgeBlockCaches() {
	cc.bodyCache.Purge()
	cc.bodyRLPCache.Purge()
	cc.blockCache.Purge()
	cc.futureBlocks.Purge()
}

func (cs *Cache) PurgeHeaderCaches() {
	cc.headerCache.Purge()
	cc.numberCache.Purge()
}

func (cs *Cache) AddHeader(hash crypto.Hash, header *types.Header) {
	cs.headerCache.Add(hash, header)
}

func (cs *Cache) AddNumber(hash crypto.Hash, number uint64) {
	cs.numberCache.Add(hash, number)
}
