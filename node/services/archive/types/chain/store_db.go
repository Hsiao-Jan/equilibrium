package chain

import (
	"crypto"

	"github.com/kowala-tech/equilibrium/database"
	"github.com/kowala-tech/equilibrium/database/rawdb"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
)

var _ BlockStore = (*Database)(nil)

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

func (ds *Database) HasHeader(hash crypto.Hash, number uint64) bool {
	return rawdb.HasHeader(ds.chainDb, hash, number)
}

func (ds *Database) GetHeader(hash crypto.Hash, number uint64) *types.Header {
	return rawdb.ReadHeader(ds.chainDb, hash, number)
}

func (ds *Database) GetBlockNumber(hash crypto.Hash) *uint64 {
	return rawdb.ReadHeaderNumber(ds.chainDb, hash)
}

func (ds *Database) GetBlock(hash crypto.Hash, number uint64) *types.Block {
	return rawdb.ReadBlock(ds.db, hash, number)
}

func (ds *Database) GetBody(hash crypto.Hash) *types.Body {
	number := ds.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}

	return rawdb.ReadBody(ds.db, hash, *number)
}

func (ds *Database) GetBodyRLP(hash crypto.Hash) rlp.RawValue {
	number := ds.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}

	return rawdb.ReadBodyRLP(ds.db, hash, *number)
}

func (ds *Database) HasBlock(hash crypto.Hash, number uint64) bool {
	return rawdb.HasBody(ds.db, hash, number)
}

func (ds *Database) PurgeBlockCaches() {}

func (ds *Database) PurgeHeaderCaches() {}

func (ds *Database) AddHeader(hash crypto.Hash, header *types.Header) {}

func (ds *Database) AddNumber(hash crypto.Hash, number uint64) {}
