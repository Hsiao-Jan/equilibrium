package types

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/kowala-tech/equilibrium/database"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
	"github.com/kowala-tech/equilibrium/services/archive/types/rawdb"
	"github.com/kowala-tech/equilibrium/types"
)

var (
	_ BlockReader = (*Cache)(nil)
	_ BlockReader = (*Database)(nil)
)

const (
	bodyCacheLimit   = 256
	blockCacheLimit  = 256
	maxFutureBlocks  = 256
	headerCacheLimit = 512
	numberCacheLimit = 2048
)

type BlockReader interface {
	HeaderReader
	GetBody(hash types.Hash) *types.Body
	GetBodyRLP(hash types.Hash) rlp.RawValue
	HasBlock(hash Hash, number uint64) bool
	GetBlock(hash Hash, number uint64) *types.Block
	PurgeBlockCaches()
}

type HeaderReader interface {
	HasHeader(has types.Hash, number uint64)
	GetHeader(hash types.Hash, number uint64) *types.Header
	GetBlockNumber(hash Hash) *uint64
	PurgeHeaderCaches()
	AddHeader(hash types.Hash, header *types.Header)
	AddNumber(hash types.Hash, number uint64)
}

type Cache struct {
	Store

	// block related caches
	bodyCache    *lru.Cache // Cache for the most recent block bodies
	bodyRLPCache *lru.Cache // Cache for the most recent block bodies in RLP encoded format
	blockCache   *lru.Cache // Cache for the most recent entire blocks
	futureBlocks *lru.Cache // future blocks are blocks added for later processing

	// header related caches
	headerCache *lru.Cache // Cache for the most recent block headers
	numberCache *lru.Cache // Cache for the most recent block numbers
}

func NewCache(store Store) *Cache {
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

func (cc *Cache) HasHeader(hash types.Hash, number uint64) *types.Header {
	if cc.numberCache.Contains(hash) || cc.headerCache.Contains(hash) {
		return true
	}
	return cc.Store.HasHeader(hc.chainDb, hash, number)
}

func (cc *Cache) GetHeader(hash types.Hash, number uint64) *types.Header {
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

func (cc *Cache) GetBlockNumber(hash types.Hash) *uint64 {
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

func (cc *Cache) GetBlock(hash types.Hash, number uint64) *types.Block {
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

func (cc *Cache) HasBlock(hash Hash, number uint64) bool {
	if cc.blockCache.Contains(hash) {
		return true
	}

	return cc.Store.HasBlock(hash, number)
}

func (cc *Cache) GetBody(hash Hash) *types.Body {
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

func (cc *Cache) GetBodyRLP(hash Hash) rlp.RawValue {
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

func (cc *Cache) PurgeBlockCaches(hash Hash) rlp.RawValue {
	cc.bodyCache.Purge()
	cc.bodyRLPCache.Purge()
	cc.blockCache.Purge()
	cc.futureBlocks.Purge()
}

func (cs *Cache) PurgeHeaderCaches(hash Hash) rlp.RawValue {
	cc.headerCache.Purge()
	cc.numberCache.Purge()
}

func (ds *Database) AddHeader(hash types.Hash, header *types.Header) {
	hc.headerCache.Add(hash, header)
}

func (ds *Database) AddNumber(hash types.Hash, number uint64) {
	hc.numberCache.Add(hash, number)
}

type Database struct {
	db database.Database
	hc *HeaderChain
}

func NewDatabase(db database.Database, hc *HeaderChain) *Database {
	return &DatabaseStore{
		db: db,
		hc: hc,
	}
}

func (ds *Database) HasHeader(hash types.Hash, number uint64) *types.Header {
	return rawdb.ReadHeader(ds.chainDb, hash, number)
}

func (ds *Database) GetHeader(hash types.Hash, number uint64) *types.Header {
	return rawdb.ReadHeader(ds.chainDb, hash, number)
}

func (ds *Database) GetBlockNumber(hash Hash) *uint64 {
	return rawdb.ReadHeaderNumber(ds.chainDb, hash)
}

func (ds *Database) GetBlock(hash Hash, number uint64) *types.Block {
	return rawdb.ReadBlock(ds.db, hash, number)
}

func (ds *Database) GetBlock(hash Hash, number uint64) *types.Block {}

func (ds *Database) GetBody(hash Hash) *types.Body {
	number := ds.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}

	return rawdb.ReadBody(ds.db, hash, *number)
}

func (ds *Database) GetBodyRLP(hash Hash) rlp.RawValue {
	number := ds.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}

	return rawdb.ReadBodyRLP(ds.db, hash, *number)
}

func (ds *Database) HasBlock(hash Hash, number uint64) bool {
	return rawdb.HasBody(ds.db, hash, number)
}

func (ds *Database) PurgeBlockCaches()                               {}
func (ds *Database) PurgeHeaderCaches()                              {}
func (ds *Database) AddHeader(hash types.Hash, header *types.Header) {}
func (ds *Database) AddNumber(hash types.Hash, number uint64)        {}
