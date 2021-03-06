package chain

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
	"sync/atomic"
	"time"

	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/database"
	"github.com/kowala-tech/equilibrium/database/rawdb"
	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
	"github.com/kowala-tech/kcoin/client/consensus"
	"go.uber.org/zap"
)

// HeaderChain implements the basic block header chain logic that is used by
// core.BlockChain - it is not usable in itself.
// It is not thread safe either, the encapsulating chain structures should do
// the necessary mutex locking/unlocking.
type HeaderChain struct {
	chainDb       database.Database
	genesisHeader *types.Header

	currentHeader     atomic.Value // Current head of the header chain (may be above the block chain!)
	currentHeaderHash crypto.Hash  // Hash of the current head of the header chain (prevent recomputing all the time)

	store HeaderStore

	procInterrupt func() bool

	rand   *mrand.Rand
	engine consensus.Engine
}

// NewHeaderChain creates a new HeaderChain structure.
//  getValidator should return the parent's validator
//  procInterrupt points to the parent's interrupt semaphore
//  wg points to the parent's shutdown wait group
func NewHeaderChain(chainDb database.Database, engine consensus.Engine, procInterrupt func() bool) (*HeaderChain, error) {

	// Seed a fast but crypto originating random generator
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	hc := &HeaderChain{
		chainDb:       chainDb,
		procInterrupt: procInterrupt,
		rand:          mrand.New(mrand.NewSource(seed.Int64())),
		engine:        engine,
	}

	hc.genesisHeader = hc.GetHeaderByNumber(0)
	if hc.genesisHeader == nil {
		return nil, ErrNoGenesis
	}

	hc.currentHeader.Store(hc.genesisHeader)
	if head := rawdb.ReadHeadBlockHash(chainDb); head != (crypto.Hash{}) {
		if chead := hc.GetHeaderByHash(head); chead != nil {
			hc.currentHeader.Store(chead)
		}
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	return hc, nil
}

// GetBlockNumber retrieves the block number belonging to the given hash
// from the cache or database
func (hc *HeaderChain) GetBlockNumber(hash crypto.Hash) *uint64 {
	return hc.store.GetBlockNumber(hash)
}

// WriteHeader writes a header into the local chain, given that its parent is
// already known.
//
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (hc *HeaderChain) WriteHeader(header *types.Header) error {
	// Cache some values to prevent constant recalculation
	var (
		hash   = header.Hash()
		number = header.Number.Uint64()
	)

	rawdb.WriteHeader(hc.chainDb, header)

	// Delete any canonical number assignments above the new head
	batch := hc.chainDb.NewBatch()
	for i := number + 1; ; i++ {
		hash := rawdb.ReadCanonicalHash(hc.chainDb, i)
		if hash == (crypto.Hash{}) {
			break
		}
		rawdb.DeleteCanonicalHash(batch, i)
	}
	batch.Write()

	// Overwrite any stale canonical number assignments
	var (
		headHash   = header.PreviousBlockHash
		headNumber = header.Number.Uint64() - 1
		headHeader = hc.GetHeader(headHash, headNumber)
	)
	if headHeader == nil {
		log.Error("error processing block head", zap.String("hash", headHash.String()), zap.Uint64("number", headNumber))
		// @TODO (rgeraldes)
		//log.Error("error processing block head. additional info", zap.String("headerByHash", hc.GetHeaderByHash(headHash).String())
		//log.Error("error processing block head. additional info", zap.String("headerByNumber", hc.GetHeaderByNumber(headNumber).String())
	}
	for rawdb.ReadCanonicalHash(hc.chainDb, headNumber) != headHash {
		rawdb.WriteCanonicalHash(hc.chainDb, headHash, headNumber)

		headHash = headHeader.PreviousBlockHash
		headNumber = headHeader.Number.Uint64() - 1
		headHeader = hc.GetHeader(headHash, headNumber)
	}
	// Extend the canonical chain with the new header
	rawdb.WriteCanonicalHash(hc.chainDb, hash, number)
	rawdb.WriteHeadHeaderHash(hc.chainDb, hash)

	hc.currentHeaderHash = hash
	hc.currentHeader.Store(types.CopyHeader(header))

	hc.store.AddHeader(hash, header)
	hc.store.AddNumber(hash, number)

	return nil
}

// WhCallback is a callback function for inserting individual headers.
// A callback is used for two reasons: first, in a LightChain, status should be
// processed and light chain events sent, while in a BlockChain this is not
// necessary since chain events are sent after inserting blocks. Second, the
// header writes should be protected by the parent chain mutex individually.
type WhCallback func(*types.Header) error

func (hc *HeaderChain) ValidateHeaderChain(chain []*types.Header, checkFreq int) (int, error) {
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if chain[i].Number.Uint64() != chain[i-1].Number.Uint64()+1 || chain[i].PreviousBlockHash != chain[i-1].Hash() {
			// Chain broke ancestry, log a message (programming error) and skip insertion
			log.Error("Non contiguous header insert", zap.String("number", chain[i].Number.String()), zap.String("hash", chain[i].Hash().String()),
				zap.String("parent", chain[i].PreviousBlockHash.String()), zap.String("prevnumber", chain[i-1].Number.String()), zap.String("prevhash", chain[i-1].Hash().String()))

			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].Number,
				chain[i-1].Hash().Bytes()[:4], i, chain[i].Number, chain[i].Hash().Bytes()[:4], chain[i].PreviousBlockHash[:4])
		}
	}

	// Generate the list of seal verification requests, and start the parallel verifier
	seals := make([]bool, len(chain))
	if checkFreq != 0 {
		// In case of checkFreq == 0 all seals are left false.
		for i := 0; i < len(seals)/checkFreq; i++ {
			index := i*checkFreq + hc.rand.Intn(checkFreq)
			if index >= len(seals) {
				index = len(seals) - 1
			}
			seals[index] = true
		}
		// Last should always be verified to avoid junk.
		seals[len(seals)-1] = true
	}

	abort, results := hc.engine.VerifyHeaders(hc, chain, seals)
	defer close(abort)

	// Iterate over the headers and ensure they all check out
	for i, header := range chain {
		// If the chain is terminating, stop processing blocks
		if hc.procInterrupt() {
			log.Debug("Premature abort during headers verification")
			return 0, errors.New("aborted")
		}
		// Otherwise wait for headers checks and ensure they pass
		if err := <-results; err != nil {
			return i, err
		}
	}

	return 0, nil
}

// InsertHeaderChain attempts to insert the given header chain in to the local
// chain, possibly creating a reorg. If an error is returned, it will return the
// index number of the failing header as well an error describing what went wrong.
//
// The verify parameter can be used to fine tune whether nonce verification
// should be done or not. The reason behind the optional check is because some
// of the header retrieval mechanisms already need to verfy nonces, as well as
// because nonces can be verified sparsely, not needing to check each.
func (hc *HeaderChain) InsertHeaderChain(chain []*types.Header, writeHeader WhCallback, start time.Time) (int, error) {
	// Collect some import statistics to report on
	stats := struct{ processed, ignored int }{}
	// All headers passed verification, import them into the database
	for i, header := range chain {
		// Short circuit insertion if shutting down
		if hc.procInterrupt() {
			log.Debug("Premature abort during headers import")
			return i, errors.New("aborted")
		}
		// If the header's already known, skip it, otherwise store
		if hc.HasHeader(header.Hash(), header.Number.Uint64()) {
			stats.ignored++
			continue
		}
		if err := writeHeader(header); err != nil {
			return i, err
		}
		stats.processed++
	}
	// Report some public statistics so the user has a clue what's going on
	last := chain[len(chain)-1]
	// @TODO (rgeraldes)
	//log.Info("Imported new block headers", "count", stats.processed, "elapsed", common.PrettyDuration(time.Since(start)),
	//	"number", last.Number, "hash", last.Hash(), "ignored", stats.ignored)

	return 0, nil
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (hc *HeaderChain) GetBlockHashesFromHash(hash crypto.Hash, max uint64) []crypto.Hash {
	// Get the origin header from which to fetch
	header := hc.GetHeaderByHash(hash)
	if header == nil {
		return nil
	}
	// Iterate the headers until enough is collected or the genesis reached
	chain := make([]crypto.Hash, 0, max)
	for i := uint64(0); i < max; i++ {
		next := header.PreviousBlockHash
		if header = hc.GetHeader(next, header.Number.Uint64()-1); header == nil {
			break
		}
		chain = append(chain, next)
		if header.Number.Sign() == 0 {
			break
		}
	}
	return chain
}

// GetAncestor retrieves the Nth ancestor of a given block. It assumes that either the given block or
// a close ancestor of it is canonical. maxNonCanonical points to a downwards counter limiting the
// number of blocks to be individually checked before we reach the canonical chain.
//
// Note: ancestor == 0 returns the same block, 1 returns its parent and so on.
func (hc *HeaderChain) GetAncestor(hash crypto.Hash, number, ancestor uint64, maxNonCanonical *uint64) (crypto.Hash, uint64) {
	if ancestor > number {
		return crypto.Hash{}, 0
	}
	if ancestor == 1 {
		// in this case it is cheaper to just read the header
		if header := hc.GetHeader(hash, number); header != nil {
			return header.PreviousBlockHash, number - 1
		} else {
			return crypto.Hash{}, 0
		}
	}
	for ancestor != 0 {
		if rawdb.ReadCanonicalHash(hc.chainDb, number) == hash {
			number -= ancestor
			return rawdb.ReadCanonicalHash(hc.chainDb, number), number
		}
		if *maxNonCanonical == 0 {
			return crypto.Hash{}, 0
		}
		*maxNonCanonical--
		ancestor--
		header := hc.GetHeader(hash, number)
		if header == nil {
			return crypto.Hash{}, 0
		}
		hash = header.PreviousBlockHash
		number--
	}
	return hash, number
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (hc *HeaderChain) GetHeader(hash crypto.Hash, number uint64) *types.Header {
	return hc.store.GetHeader(hash, number)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (hc *HeaderChain) GetHeaderByHash(hash crypto.Hash) *types.Header {
	number := hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return hc.GetHeader(hash, *number)
}

// HasHeader checks if a block header is present in the database or not.
func (hc *HeaderChain) HasHeader(hash crypto.Hash, number uint64) bool {
	return hc.store.HasHeader(hash, number)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (hc *HeaderChain) GetHeaderByNumber(number uint64) *types.Header {
	hash := rawdb.ReadCanonicalHash(hc.chainDb, number)
	if hash == (crypto.Hash{}) {
		return nil
	}
	return hc.GetHeader(hash, number)
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (hc *HeaderChain) CurrentHeader() *types.Header {
	return hc.currentHeader.Load().(*types.Header)
}

// SetCurrentHeader sets the current head header of the canonical chain.
func (hc *HeaderChain) SetCurrentHeader(head *types.Header) {
	rawdb.WriteHeadHeaderHash(hc.chainDb, head.Hash())

	hc.currentHeader.Store(head)
	hc.currentHeaderHash = head.Hash()
}

// DeleteCallback is a callback function that is called by RewindTo before
// each header is deleted.
type DeleteCallback func(rawdb.DatabaseDeleter, crypto.Hash, uint64)

// RewindTorewinds the local chain to a new head. Everything above the new head
// will be deleted and the new one set.
func (hc *HeaderChain) RewindTo(head uint64, delFn DeleteCallback) {
	height := uint64(0)

	if hdr := hc.CurrentHeader(); hdr != nil {
		height = hdr.Number.Uint64()
	}
	batch := hc.chainDb.NewBatch()
	for hdr := hc.CurrentHeader(); hdr != nil && hdr.Number.Uint64() > head; hdr = hc.CurrentHeader() {
		hash := hdr.Hash()
		num := hdr.Number.Uint64()
		if delFn != nil {
			delFn(batch, hash, num)
		}
		rawdb.DeleteHeader(batch, hash, num)

		hc.currentHeader.Store(hc.GetHeader(hdr.PreviousBlockHash, hdr.Number.Uint64()-1))
	}
	// Roll back the canonical chain numbering
	for i := height; i > head; i-- {
		rawdb.DeleteCanonicalHash(batch, i)
	}
	batch.Write()

	// Clear out any stale content from the caches
	hc.store.PurgeHeaderCaches()

	if hc.CurrentHeader() == nil {
		hc.currentHeader.Store(hc.genesisHeader)
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	rawdb.WriteHeadHeaderHash(hc.chainDb, hc.currentHeaderHash)
}

// SetGenesis sets a new genesis block header for the chain
func (hc *HeaderChain) SetGenesis(head *types.Header) {
	hc.genesisHeader = head
}

// Engine retrieves the header chain's consensus engine.
func (hc *HeaderChain) Engine() consensus.Engine { return hc.engine }

// GetBlock implements consensus.ChainReader, and returns nil for every input as
// a header chain does not have blocks available for retrieval.
func (hc *HeaderChain) GetBlock(hash crypto.Hash, number uint64) *types.Block {
	return nil
}
