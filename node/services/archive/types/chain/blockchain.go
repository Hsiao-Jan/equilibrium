package chain

import (
	"github.com/kowala-tech/equilibrium/crypto/signer"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/golang-lru"
	"github.com/kowala-tech/equilibrium/common"
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/database"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/node/event"
	"github.com/kowala-tech/equilibrium/database/rawdb"
	"github.com/kowala-tech/equilibrium/state"
	"github.com/kowala-tech/equilibrium/state/accounts"
	"github.com/kowala-tech/equilibrium/state/accounts/transaction"
	"github.com/kowala-tech/equilibrium/state/trie"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
	"github.com/kowala-tech/equilibrium/vm"
	"github.com/kowala-tech/kcoin/client/consensus"
	"go.uber.org/zap"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)


var (
//blockInsertTimer = metrics.NewRegisteredTimer("chain/inserts", nil)
)

const (
	maxTimeFutureBlocks = 30
	badBlockLimit       = 10
	triesInMemory       = 128

	// BlockChainVersion ensures that an incompatible database forces a resync from scratch.
	// BlockChainVersion = 1
)

// Validator is an interface which defines the standard for block validation. It
// is only responsible for validating block contents, as the header validation is
// done by the specific consensus engines.
//
type Validator interface {
	// ValidateBody validates the given block's content.
	ValidateBody(block *types.Block) error

	// ValidateState validates the given statedb and optionally the receipts and
	// gas used.
	ValidateState(block, parent *types.Block, state *state.StateDB, receipts transaction.Receipts, usedGas uint64) error
}

// Processor is an interface for processing blocks using a given initial state.
//
// Process takes the block to be processed and the statedb upon which the
// initial state is based. It should return the receipts generated, amount
// of gas used in the process and return an error if any of the internal rules
// failed.
type Processor interface {
	Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (transaction.Receipts, []*transaction.Log, uint64, error)
}

// BlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain imports, reverts, chain reorganisations.
// The BlockChain also helps in returning blocks from the canonical chain.
type BlockChain struct {
	mu      sync.RWMutex // global mutex for locking chain operations.
	chainmu sync.RWMutex // blockchain insertion lock.
	procmu  sync.RWMutex // block processor lock.

	cacheConfig *CacheConfig    // Cache configuration for pruning.

	running          int32
	genesisBlock     *types.Block
	currentBlock     atomic.Value // Current head of the block chain.
	currentFastBlock atomic.Value // Current head of the fast-sync chain (may be above the block chain!).
	hc               *HeaderChain

	db     database.Database // Low level persistent database to store final content in.
	triegc *prque.Prque      // Priority queue mapping block numbers to tries to gc.
	gcproc time.Duration     // Accumulates canonical block processing for trie dumping.

	store BlockStore

	scope         event.SubscriptionScope
	rmLogsFeed    event.Feed
	chainFeed     event.Feed
	chainHeadFeed event.Feed
	logsFeed      event.Feed

	checkpoint int // checkpoint counts towards the new checkpoint

	futureBlocks *lru.Cache    // future blocks are blocks added for later processing
	stateCache state.Database // State database to reuse between imports (contains state cache)

	// interrupt must be atomically called
	interrupt int32          // interrupt signaler for block processing
	wg        sync.WaitGroup // chain processing wait group for shutting down

	engine    consensus.Engine
	processor Processor // block processor interface
	validator Validator // block and state validator interface
	vmConfig  vm.Config

	badBlocks *lru.Cache // Bad block cache

	doneCh chan struct{}
}

// New returns a fully initialised block chain using information
// available in the database.
func New(db database.Database, cacheConfig *CacheConfig, engine consensus.Engine, vmConfig vm.Config) (*BlockChain, error) {
	if cacheConfig == nil {
		cacheConfig = &CacheConfig{
			TrieNodeLimit: 256 * 1024 * 1024,
			TrieTimeLimit: 5 * time.Minute,
		}
	}

	badBlocks, _ := lru.New(badBlockLimit)

	bc := &BlockChain{
		cacheConfig: cacheConfig,
		db:          db,
		triegc:      prque.New(),
		stateCache:  state.NewDatabase(db),
		engine:      engine,
		vmConfig:    vmConfig,
		badBlocks:   badBlocks,
		doneCh:      make(chan struct{}),
	}
	
	// @TODO (rgeraldes)
	//bc.SetValidator(NewBlockValidator(bc, engine))
	//bc.SetProcessor(NewStateProcessor(bc, engine))

	var err error
	store := NewCache(NewDatabase(bc.db, bc.hc))
	bc.hc, err = NewHeaderChain(db, engine, bc.getProcInterrupt)
	if err != nil {
		return nil, err
	}
	bc.store = store

	bc.genesisBlock = bc.GetBlockByNumber(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}
	if err := bc.loadState(); err != nil {
		return nil, err
	}

	// Take ownership of this particular state
	go bc.update()
	return bc, nil
}

func (bc *BlockChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&bc.interrupt) == 1
}

// loadState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *BlockChain) loadState() error {
	// Restore the last known head block
	head := rawdb.ReadHeadBlockHash(bc.db)
	if head == (crypto.Hash{}) {
		// Corrupt or empty database, init from scratch
		log.Info("Empty database, resetting chain")
		return bc.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := bc.GetBlockByHash(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		log.Info("Head block missing, resetting chain", zap.String("hash", head.String()))
		return bc.Reset()
	}
	// Make sure the state associated with the block is available
	if _, err := state.New(currentBlock.Snapshot(), bc.stateCache); err != nil {
		// Dangling block without a state associated, init from scratch
		log.Error("Head state missing, repairing chain", zap.Uint64("number", currentBlock.Number().Uint64()), zap.String("hash", currentBlock.Hash().String()))
		if err := bc.repair(&currentBlock); err != nil {
			return err
		}
	}
	// Everything seems to be fine, set as the head block
	bc.currentBlock.Store(currentBlock)

	// Restore the last known head header
	currentHeader := currentBlock.Header()
	if head := rawdb.ReadHeadHeaderHash(bc.db); head != (crypto.Hash{}) {
		if header := bc.GetHeaderByHash(head); header != nil {
			currentHeader = header
		}
	}
	bc.hc.SetCurrentHeader(currentHeader)

	// Restore the last known head fast block
	bc.currentFastBlock.Store(currentBlock)
	if head := rawdb.ReadHeadFastBlockHash(bc.db); head != (crypto.Hash{}) {
		if block := bc.GetBlockByHash(head); block != nil {
			bc.currentFastBlock.Store(block)
		}
	}

	// Issue a status log for the user
	log.Debug("Loaded most recent local header", zap.String("number", currentHeader.Number.String()), zap.String("hash", currentHeader.Hash().String()))
	log.Debug("Loaded most recent local full block", zap.String("number", currentBlock.Number().String()), zap.String("hash", currentBlock.Hash().String()))
	log.Debug("Loaded most recent local fast block", zap.String("number", bc.CurrentFastBlock().Number().String()), zap.String("hash", bc.CurrentFastBlock().Hash().String()))

	return nil
}

// RewindTo rewinds the local chain to a new head. In the case of headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
func (bc *BlockChain) RewindTo(head uint64) error {
	log.Info("Rewinding blockchain", zap.Uint64("target", head))

	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Rewind the header chain, deleting all block bodies until then
	delFn := func(db rawdb.DatabaseDeleter, hash crypto.Hash, num uint64) {
		rawdb.DeleteBody(db, hash, num)
	}
	bc.hc.RewindTo(head, delFn)
	currentHeader := bc.hc.CurrentHeader()

	// Clear out any stale content from the caches
	bc.store.PurgeBlockCaches()

	// Rewind the block chain, ensuring we don't end up with a stateless head block
	if currentBlock := bc.CurrentBlock(); currentBlock != nil && currentHeader.Number.Uint64() < currentBlock.NumberU64() {
		bc.currentBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Number.Uint64()))
	}
	if currentBlock := bc.CurrentBlock(); currentBlock != nil {
		if _, err := state.New(currentBlock.Snapshot(), bc.stateCache); err != nil {
			// Rewound state missing, rolled back to before pivot, reset to genesis
			bc.currentBlock.Store(bc.genesisBlock)
		}
	}
	// Rewind the fast block in a simpleton way to the target head
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock != nil && currentHeader.Number.Uint64() < currentFastBlock.NumberU64() {
		bc.currentFastBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Number.Uint64()))
	}
	// If either blocks reached nil, reset to the genesis state
	if currentBlock := bc.CurrentBlock(); currentBlock == nil {
		bc.currentBlock.Store(bc.genesisBlock)
	}
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock == nil {
		bc.currentFastBlock.Store(bc.genesisBlock)
	}
	currentBlock := bc.CurrentBlock()
	currentFastBlock := bc.CurrentFastBlock()

	rawdb.WriteHeadBlockHash(bc.db, currentBlock.Hash())
	rawdb.WriteHeadFastBlockHash(bc.db, currentFastBlock.Hash())

	return bc.loadState()
}

// FastSyncCommitHead sets the current head block to the one defined by the hash
// irrelevant what the chain contents were prior.
func (bc *BlockChain) FastSyncCommitHead(hash crypto.Hash) error {
	// Make sure that both the block as well at its state trie exists
	block := bc.GetBlockByHash(hash)
	if block == nil {
		return fmt.Errorf("non existent block [%x…]", hash[:4])
	}
	if _, err := trie.NewSecure(block.Snapshot(), bc.stateCache.TrieDB(), 0); err != nil {
		return err
	}
	// If all checks out, manually set the head block
	bc.mu.Lock()
	bc.currentBlock.Store(block)
	bc.mu.Unlock()

	log.Info("Committed new head block", zap.String("number", block.Number().String()), zap.String("hash", hash.String()))
	return nil
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (bc *BlockChain) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

// CurrentFastBlock retrieves the current fast-sync head block of the canonical
// chain. The block is retrieved from the blockchain's internal cache.
func (bc *BlockChain) CurrentFastBlock() *types.Block {
	return bc.currentFastBlock.Load().(*types.Block)
}

// SetProcessor sets the processor required for making state modifications.
func (bc *BlockChain) SetProcessor(processor Processor) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.processor = processor
}

// SetValidator sets the validator which is used to validate incoming blocks.
func (bc *BlockChain) SetValidator(validator Validator) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.validator = validator
}

// Validator returns the current validator.
func (bc *BlockChain) Validator() Validator {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.validator
}

// Processor returns the current processor.
func (bc *BlockChain) Processor() Processor {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.processor
}

// State returns a new mutable state based on the current HEAD block.
func (bc *BlockChain) State() (*state.StateDB, error) {
	return bc.StateAt(bc.CurrentBlock().Snapshot())
}

// StateAt returns a new mutable state based on a particular point in time.
func (bc *BlockChain) StateAt(root crypto.Hash) (*state.StateDB, error) {
	return state.New(root, bc.stateCache)
}

// Reset purges the entire blockchain, restoring it to its genesis state.
func (bc *BlockChain) Reset() error {
	return bc.ResetWithGenesisBlock(bc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (bc *BlockChain) ResetWithGenesisBlock(genesis *types.Block) error {
	// Dump the entire block chain and purge the caches
	if err := bc.RewindTo(0); err != nil {
		return err
	}
	bc.mu.Lock()
	defer bc.mu.Unlock()

	rawdb.WriteBlock(bc.db, genesis)

	bc.genesisBlock = genesis
	bc.insert(bc.genesisBlock)
	bc.currentBlock.Store(bc.genesisBlock)
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	bc.hc.SetCurrentHeader(bc.genesisBlock.Header())
	bc.currentFastBlock.Store(bc.genesisBlock)

	return nil
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (bc *BlockChain) repair(head **types.Block) error {
	for {
		// Abort if we've rewound to a head block that does have associated state
		if _, err := state.New((*head).Snapshot(), bc.stateCache); err == nil {
			log.Info("Rewound blockchain to past state", zap.String("number", (*head).Number().String()), zap.String("hash", (*head).Hash().String()))
			return nil
		}
		// Otherwise rewind one block and recheck state availability there
		(*head) = bc.GetBlock((*head).PreviousBlockHash(), (*head).NumberU64()-1)
	}
}

// Export writes the active chain to the given writer.
func (bc *BlockChain) Export(w io.Writer) error {
	return bc.ExportN(w, uint64(0), bc.CurrentBlock().NumberU64())
}

// ExportN writes a subset of the active chain to the given writer.
func (bc *BlockChain) ExportN(w io.Writer, first uint64, last uint64) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if first > last {
		return fmt.Errorf("export failed: first (%d) is greater than last (%d)", first, last)
	}
	log.Debug("Exporting batch of blocks", zap.Uint64("count", last-first+1))

	for nr := first; nr <= last; nr++ {
		block := bc.GetBlockByNumber(nr)
		if block == nil {
			return fmt.Errorf("export failed on #%d: not found", nr)
		}

		if err := block.EncodeRLP(w); err != nil {
			return err
		}
	}

	return nil
}

// insert injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header and the head fast sync block to this very same block if they are older.
// Note, this function assumes that the `mu` mutex is held!
func (bc *BlockChain) insert(block *types.Block) {
	// unknown block
	updateHeads := rawdb.ReadCanonicalHash(bc.db, block.NumberU64()) != block.Hash()

	// Add the block to the canonical chain number scheme and mark as the head
	rawdb.WriteCanonicalHash(bc.db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(bc.db, block.Hash())

	bc.currentBlock.Store(block)

	// If the block is better than our head, force update heads
	if updateHeads {
		bc.hc.SetCurrentHeader(block.Header())
		rawdb.WriteHeadFastBlockHash(bc.db, block.Hash())

		bc.currentFastBlock.Store(block)
	}
}

// Genesis retrieves the chain's genesis block.
func (bc *BlockChain) Genesis() *types.Block {
	return bc.genesisBlock
}

// GetBody retrieves a block body (transactions, pre-commits and protocol violations)
// from the database by hash, caching it if found.
func (bc *BlockChain) GetBody(hash crypto.Hash) *types.Body {
	return bc.store.GetBody(hash)
}

// GetBodyRLP retrieves a block body in RLP encoding from the database by hash,
// caching it if found.
func (bc *BlockChain) GetBodyRLP(hash crypto.Hash) rlp.RawValue {
	return bc.store.GetBodyRLP(hash)
}

// HasBlock checks if a block is fully present in the database or not.
func (bc *BlockChain) HasBlock(hash crypto.Hash, number uint64) bool {
	return bc.store.HasBlock(hash, number)
}

// HasState checks if state trie is fully present in the database or not.
func (bc *BlockChain) HasState(hash crypto.Hash) bool {
	_, err := bc.stateCache.OpenTrie(hash)
	return err == nil
}

// HasBlockAndState checks if a block and associated state trie is fully present
// in the database or not, caching it if present.
func (bc *BlockChain) HasBlockAndState(hash crypto.Hash, number uint64) bool {
	// Check first that the block itself is known
	block := bc.GetBlock(hash, number)
	if block == nil {
		return false
	}
	return bc.HasState(block.Snapshot())
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (bc *BlockChain) GetBlock(hash crypto.Hash, number uint64) *types.Block {
	return bc.store.GetBlock(hash, number)
}

// GetBlockByHash retrieves a block from the database by hash, caching it if found.
func (bc *BlockChain) GetBlockByHash(hash crypto.Hash) *types.Block {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetBlock(hash, *number)
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (bc *BlockChain) GetBlockByNumber(number uint64) *types.Block {
	hash := rawdb.ReadCanonicalHash(bc.db, number)
	if hash == (crypto.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, number)
}

// GetReceiptsByHash retrieves the receipts for all transactions in a given block.
func (bc *BlockChain) GetReceiptsByHash(hash crypto.Hash) transaction.Receipts {
	number := rawdb.ReadHeaderNumber(bc.db, hash)
	if number == nil {
		return nil
	}
	return rawdb.ReadReceipts(bc.db, hash, *number)
}

// GetBlocksFromHash returns the block corresponding to hash and up to n-1 ancestors.
// [deprecated by eth/62]
func (bc *BlockChain) GetBlocksFromHash(hash crypto.Hash, n int) (blocks []*types.Block) {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	for i := 0; i < n; i++ {
		block := bc.GetBlock(hash, *number)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
		hash = block.PreviousBlockHash()
		*number--
	}
	return
}

// TrieNode retrieves a blob of data associated with a trie node (or code hash)
// either from ephemeral in-memory cache, or from persistent storage.
func (bc *BlockChain) TrieNode(hash crypto.Hash) ([]byte, error) {
	return bc.stateCache.TrieDB().Node(hash)
}

// Stop stops the blockchain service. If any imports are currently in progress
// it will abort them using the interrupt channel.
func (bc *BlockChain) Stop() {
	if !atomic.CompareAndSwapInt32(&bc.running, 0, 1) {
		return
	}
	// Unsubscribe all subscriptions registered from blockchain
	bc.scope.Close()
	close(bc.doneCh)
	atomic.StoreInt32(&bc.interrupt, 1)

	bc.wg.Wait()

	// Ensure the state of a recent block is also stored to disk before exiting.
	// We're writing three different states to catch different restart scenarios:
	//  - HEAD:     So we don't need to reprocess any blocks in the general case
	//  - HEAD-1:   So we don't do large reorgs if our HEAD becomes an uncle
	//  - HEAD-127: So we have a hard limit on the number of blocks reexecuted
	if !bc.cacheConfig.Disabled {
		triedb := bc.stateCache.TrieDB()

		for _, offset := range []uint64{0, 1, triesInMemory - 1} {
			if number := bc.CurrentBlock().NumberU64(); number > offset {
				recent := bc.GetBlockByNumber(number - offset)

				log.Info("Writing cached state to disk", zap.String("block", recent.Number().String()), zap.String("hash", recent.Hash().String()), zap.String("root", recent.Snapshot().String()))
				if err := triedb.Commit(recent.Snapshot(), true); err != nil {
					log.Error("Failed to commit recent state trie", zap.Error(err))
				}
			}
		}
		for !bc.triegc.Empty() {
			triedb.Dereference(bc.triegc.PopItem().(crypto.Hash))
		}
		if size, _ := triedb.Size(); size != 0 {
			log.Error("Dangling trie nodes after full cleanup")
		}
	}
	log.Info("Blockchain manager stopped")
}

func (bc *BlockChain) procFutureBlocks() {
	blocks := make([]*types.Block, 0, bc.futureBlocks.Len())
	for _, hash := range bc.futureBlocks.Keys() {
		if block, exist := bc.futureBlocks.Peek(hash); exist {
			blocks = append(blocks, block.(*types.Block))
		}
	}
	if len(blocks) > 0 {
		types.BlockBy(types.Number).Sort(blocks)

		// Insert one by one as chain insertion needs contiguous ancestry between blocks
		for i := range blocks {
			bc.InsertChain(blocks[i : i+1])
		}
	}
}

// WriteStatus status of write
type WriteStatus byte

const (
	NonStatTy WriteStatus = iota
	CanonStatTy
)

// Rollback is designed to remove a chain of links from the database that aren't
// certain enough to be valid.
func (bc *BlockChain) Rollback(chain []crypto.Hash) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		currentHeader := bc.hc.CurrentHeader()
		if currentHeader.Hash() == hash {
			bc.hc.SetCurrentHeader(bc.GetHeader(currentHeader.PreviousBlockHash, currentHeader.Number.Uint64()-1))
		}
		if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock.Hash() == hash {
			newFastBlock := bc.GetBlock(currentFastBlock.PreviousBlockHash(), currentFastBlock.NumberU64()-1)
			bc.currentFastBlock.Store(newFastBlock)
			rawdb.WriteHeadFastBlockHash(bc.db, newFastBlock.Hash())
		}
		if currentBlock := bc.CurrentBlock(); currentBlock.Hash() == hash {
			newBlock := bc.GetBlock(currentBlock.PreviousBlockHash(), currentBlock.NumberU64()-1)
			bc.currentBlock.Store(newBlock)
			rawdb.WriteHeadBlockHash(bc.db, newBlock.Hash())
		}
	}
}

// SetReceiptsData computes all the non-consensus fields of the receipts
func SetReceiptsData(block *types.Block, receipts transaction.Receipts) error {
	signer := signer.NewProductionSigner()

	transactions, logIndex := block.Transactions(), uint(0)
	if len(transactions) != len(receipts) {
		return errors.New("transaction and receipt count mismatch")
	}

	for j := 0; j < len(receipts); j++ {
		// The transaction hash can be retrieved from the transaction itself
		receipts[j].TxHash = transactions[j].Hash()

		// The contract address can be derived from the transaction itself
		if transactions[j].To() == nil {
			// Deriving the signer is expensive, only do if it's actually needed
			from, _ := types.TxSender(signer, transactions[j])
			receipts[j].ContractAddress = accounts.CreateAddress(from, transactions[j].Nonce())
		}

		// The derived log fields can simply be set from the block and transaction
		for k := 0; k < len(receipts[j].Logs); k++ {
			receipts[j].Logs[k].BlockNumber = block.NumberU64()
			receipts[j].Logs[k].BlockHash = block.Hash()
			receipts[j].Logs[k].TxHash = receipts[j].TxHash
			receipts[j].Logs[k].TxIndex = uint(j)
			receipts[j].Logs[k].Index = logIndex
			logIndex++
		}
	}
	return nil
}

// InsertReceiptChain attempts to complete an already existing header chain with
// transaction and receipt data.
func (bc *BlockChain) InsertReceiptChain(blockChain types.Blocks, receiptChain []transaction.Receipts) (int, error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(blockChain); i++ {
		if blockChain[i].NumberU64() != blockChain[i-1].NumberU64()+1 || blockChain[i].PreviousBlockHash() != blockChain[i-1].Hash() {
			log.Error("Non contiguous receipt insert", zap.String("number", blockChain[i].Number().String()), zap.String("hash", blockChain[i].Hash().String()), zap.String("parent", blockChain[i].PreviousBlockHash().String()),
				zap.String("prevnumber", blockChain[i-1].Number().String()), zap.String("prevhash", blockChain[i-1].Hash().String()))
			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, blockChain[i-1].NumberU64(),
				blockChain[i-1].Hash().Bytes()[:4], i, blockChain[i].NumberU64(), blockChain[i].Hash().Bytes()[:4], blockChain[i].PreviousBlockHash().Bytes()[:4])
		}
	}

	var (
		stats = struct{ processed, ignored int32 }{}
		start = time.Now()
		bytes = 0
		batch = bc.db.NewBatch()
	)
	for i, block := range blockChain {
		receipts := receiptChain[i]
		// Short circuit insertion if shutting down or processing failed
		if atomic.LoadInt32(&bc.interrupt) == 1 {
			return 0, nil
		}
		// Short circuit if the owner header is unknown
		if !bc.HasHeader(block.Hash(), block.NumberU64()) {
			return i, fmt.Errorf("containing header #%d [%x…] unknown", block.Number(), block.Hash().Bytes()[:4])
		}
		// Skip if the entire data is already known
		if bc.HasBlock(block.Hash(), block.NumberU64()) {
			stats.ignored++
			continue
		}
		// Compute all the non-consensus fields of the receipts
		if err := SetReceiptsData(block, receipts); err != nil {
			return i, fmt.Errorf("failed to set receipts data: %v", err)
		}
		// Write all the data out into the database
		rawdb.WriteBody(batch, block.Hash(), block.NumberU64(), block.Body())
		rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)
		rawdb.WriteTxLookupEntries(batch, block)

		stats.processed++

		if batch.ValueSize() >= database.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return 0, err
			}
			bytes += batch.ValueSize()
			batch.Reset()
		}
	}
	if batch.ValueSize() > 0 {
		bytes += batch.ValueSize()
		if err := batch.Write(); err != nil {
			return 0, err
		}
	}

	// Update the head fast sync block if better
	bc.mu.Lock()
	head := blockChain[len(blockChain)-1]
	if blockNumber := head.Number(); blockNumber != nil { // Rewind may have occurred, skip in that case
		if bc.CurrentFastBlock().Number().Cmp(blockNumber) < 0 {
			rawdb.WriteHeadFastBlockHash(bc.db, head.Hash())
			bc.currentFastBlock.Store(head)
		}
	}
	bc.mu.Unlock()

	log.Info("Imported new block receipts",
		zap.Int32("count", stats.processed),
		zap.String("elapsed", common.PrettyDuration(time.Since(start)).String()),
		zap.String("number", head.Number().String()),
		zap.String("hash", head.Hash().String()),
		zap.Float64("size", float64(common.StorageSize(bytes))),
		zap.Int32("ignored", stats.ignored))
	return 0, nil
}

var lastWrite uint64

// WriteBlockWithState writes the block and all associated state to the database.
// Note: WriteBlockWithState assumes that the block has been validated already.
func (bc *BlockChain) WriteBlockWithState(block *types.Block, receipts []*transaction.Receipt, state *state.StateDB) (err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	// Make sure no inconsistent state is leaked during insertion
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// write the block itself to the database
	rawdb.WriteBlock(bc.db, block)

	root, err := state.Commit(true)
	if err != nil {
		return err
	}
	triedb := bc.stateCache.TrieDB()

	// If we're running an archive node, always flush
	if bc.cacheConfig.Disabled {
		if err := triedb.Commit(root, false); err != nil {
			return err
		}
	} else {
		// Full but not archive node, do proper garbage collection
		triedb.Reference(root, crypto.Hash{}) // metadata reference to keep trie alive
		bc.triegc.Push(root, -float32(block.NumberU64()))

		if current := block.NumberU64(); current > triesInMemory {
			// If we exceeded our memory allowance, flush matured singleton nodes to disk
			var (
				nodes, imgs = triedb.Size()
				limit       = common.StorageSize(bc.cacheConfig.TrieNodeLimit) * 1024 * 1024
			)
			if nodes > limit || imgs > 4*1024*1024 {
				triedb.Cap(limit - database.IdealBatchSize)
			}
			// Find the next state trie we need to commit
			header := bc.GetHeaderByNumber(current - triesInMemory)
			chosen := header.Number.Uint64()

			// If we exceeded out time allowance, flush an entire trie to disk
			if bc.gcproc > bc.cacheConfig.TrieTimeLimit {
				// If we're exceeding limits but haven't reached a large enough memory gap,
				// warn the user that the system is becoming unstable.
				if chosen < lastWrite+triesInMemory && bc.gcproc >= 2*bc.cacheConfig.TrieTimeLimit {
					log.Info("State in memory for too long, committing", zap.Duration("time", bc.gcproc), zap.Duration("allowance", bc.cacheConfig.TrieTimeLimit), zap.Float64("optimum", float64(chosen-lastWrite)/triesInMemory))
				}
				// Flush an entire trie and restart the counters
				triedb.Commit(header.Snapshot, true)
				lastWrite = chosen
				bc.gcproc = 0
			}
			// Garbage collect anything below our required write retention
			for !bc.triegc.Empty() {
				root, number := bc.triegc.Pop()
				if uint64(-number) > chosen {
					bc.triegc.Push(root, number)
					break
				}
				triedb.Dereference(root.(crypto.Hash))
			}
		}
	}

	// Write other block data using a batch.
	batch := bc.db.NewBatch()
	rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)

	// Write the positional metadata for transaction/receipt lookups and preimages
	rawdb.WriteTxLookupEntries(batch, block)
	rawdb.WritePreimages(batch, block.NumberU64(), state.Preimages())

	if err := batch.Write(); err != nil {
		return err
	}

	bc.insert(block)

	bc.futureBlocks.Remove(block.Hash())

	return nil
}

// writePositionalMetadata Write the positional metadata for transaction/receipt lookups and preimages
func writePositionalMetadata(batch database.Batch, block *types.Block, state *state.StateDB) {
	rawdb.WriteTxLookupEntries(batch, block)
	rawdb.WritePreimages(batch, block.NumberU64(), state.Preimages())
}

// InsertChain attempts to insert the given batch of blocks in to the canonical
// chain. If an error is returned it will return the index number of the failing
// block as well an error describing what went wrong.
//
// After insertion is done, all accumulated events will be fired.
func (bc *BlockChain) InsertChain(chain types.Blocks) (int, error) {
	n, events, logs, err := bc.insertChain(chain)
	bc.PostChainEvents(events, logs)
	return n, err
}

// insertChain will execute the actual chain insertion and event aggregation. The
// only reason this method exists as a separate one is to make locking cleaner
// with deferred statements.
func (bc *BlockChain) insertChain(chain types.Blocks) (int, []interface{}, []*transaction.Log, error) {
	// Sanity check that we have something meaningful to import
	if len(chain) == 0 {
		return 0, nil, nil, nil
	}
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if chain[i].NumberU64() != chain[i-1].NumberU64()+1 || chain[i].PreviousBlockHash() != chain[i-1].Hash() {
			// Chain broke ancestry, log a message (programming error) and skip insertion
			log.Error("Non contiguous block insert", zap.String("number", chain[i].Number().String()), zap.String("hash", chain[i].Hash().String()),
				zap.String("previous block", chain[i].PreviousBlockHash().String()), zap.String("prevnumber", chain[i-1].Number().String()), zap.String("prevhash", chain[i-1].Hash().String()))

			return 0, nil, nil, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].NumberU64(),
				chain[i-1].Hash().Bytes()[:4], i, chain[i].NumberU64(), chain[i].Hash().Bytes()[:4], chain[i].PreviousBlockHash().Bytes()[:4])
		}
	}
	// Pre-checks passed, start the full block imports
	bc.wg.Add(1)
	defer bc.wg.Done()

	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	// A queued approach to delivering events. This is generally
	// faster than direct delivery and requires much less mutex
	// acquiring.
	var (
		//stats         = insertStats{startTime: mclock.Now()}
		events        = make([]interface{}, 0, len(chain))
		lastCanon     *types.Block
		coalescedLogs []*transaction.Log
	)
	// Start the parallel header verifier
	headers := make([]*types.Header, len(chain))
	seals := make([]bool, len(chain))

	for i, block := range chain {
		headers[i] = block.Header()
		seals[i] = true
	}
	abort, results := bc.engine.VerifyHeaders(bc, headers, seals)
	defer close(abort)

	// Start a parallel signature recovery (signer will fluke on fork transition, minimal perf loss)
	senderCacher.recoverFromBlocks(signer.NewProductionSigner(), chain)

	// Iterate over the blocks and insert when the verifier permits
	for i, block := range chain {
		// If the chain is terminating, stop processing blocks
		if atomic.LoadInt32(&bc.interrupt) == 1 {
			log.Debug("Premature abort during blocks processing")
			break
		}
		// Wait for the block's verification to complete
		bstart := time.Now()

		err := <-results
		if err == nil {
			err = bc.Validator().ValidateBody(block)
		}
		switch {
		case err == ErrKnownBlock:
			// Block and state both already known. However if the current block is below
			// this number we did a rollback and we should reimport it nonetheless.
			if bc.CurrentBlock().NumberU64() >= block.NumberU64() {
				//stats.ignored++
				continue
			}

		case err == consensus.ErrFutureBlock:
			// Allow up to MaxFuture second in the future blocks. If this limit is exceeded
			// the chain is discarded and processed at a later time if given.
			max := big.NewInt(time.Now().Unix() + maxTimeFutureBlocks)
			if block.Time().Cmp(max) > 0 {
				return i, events, coalescedLogs, fmt.Errorf("future block: %v > %v", block.Time(), max)
			}
			bc.futureBlocks.Add(block.Hash(), block)
			//stats.queued++
			continue

		case err == consensus.ErrUnknownAncestor && bc.futureBlocks.Contains(block.PreviousBlockHash()):
			bc.futureBlocks.Add(block.Hash(), block)
			//stats.queued++
			continue
		case err != nil:
			log.Error("insert chain error while bc.engine.VerifyHeaders of a block", zap.String("err", err.Error()))
			bc.reportBlock(block, nil, err)
			return i, events, coalescedLogs, err
		}
		// Create a new statedb using the parent block and report an
		// error if it fails.
		var parent *types.Block
		if i == 0 {
			parent = bc.GetBlock(block.PreviousBlockHash(), block.NumberU64()-1)
		} else {
			parent = chain[i-1]
		}
		state, err := state.New(parent.Snapshot(), bc.stateCache)
		if err != nil {
			return i, events, coalescedLogs, err
		}
		// Process block using the parent state as reference point.
		receipts, logs, requiredEffort, err := bc.processor.Process(block, state, bc.vmConfig)
		if err != nil {
			log.Debug("insert chain error while bc.processor.Process of a block", zap.String("err", err.Error()))
			bc.reportBlock(block, receipts, err)
			return i, events, coalescedLogs, err
		}
		// Validate the state using the default validator
		err = bc.Validator().ValidateState(block, parent, state, receipts, requiredEffort)
		if err != nil {
			log.Debug("insert chain error while bc.Validator().ValidateState of a block", zap.String("data", spew.Sdump(
				// @TODO (rgeraldes)
				//block.ReceivedFrom,
				//block.ReceivedAt,
				block.Header().Number,
				block.Header().PreviousBlockHash,
				block.Header().Extra,
				block.Header().LastCommitHash,
				block.Header().TxHash,
				block.Header().Snapshot,
				parent.Header().Number,
				parent.Header().ProtocolViolationHash,
				parent.Header().TxHash,
				requiredEffort)))

			bc.reportBlock(block, receipts, err)
			return i, events, coalescedLogs, err
		}
		proctime := time.Since(bstart)

		// Write the block to the chain and get the status.
		if err := bc.WriteBlockWithState(block, receipts, state); err != nil {
			return i, events, coalescedLogs, err
		}

		log.Debug("Inserted new block", zap.String("number", block.Number().String()), zap.String("hash", block.Hash().String()),
			zap.Int("txs", len(block.Transactions())), zap.String("elapsed", common.PrettyDuration(time.Since(bstart)).String()))

		coalescedLogs = append(coalescedLogs, logs...)
		//blockInsertTimer.UpdateSince(bstart)
		events = append(events, ChainEvent{block, block.Hash(), logs})
		lastCanon = block

		bc.gcproc += proctime

		// @TODO (rgeraldes)
		//stats.processed++
		//stats.usedGas += usedGas

		cache, _ := bc.stateCache.TrieDB().Size()
		//stats.report(chain, i, cache)
	}
	// Append a single chain head event if we've progressed the chain
	if lastCanon != nil && bc.CurrentBlock().Hash() == lastCanon.Hash() {
		events = append(events, ChainHeadEvent{lastCanon})
	}
	return 0, events, coalescedLogs, nil
}

/*
// insertStats tracks and reports on block insertion.
type insertStats struct {
	queued, processed, ignored int
	usedGas                    uint64
	lastIndex                  int
	startTime                  mclock.AbsTime
}

// statsReportLimit is the time limit during import and export after which we
// always print out progress. This avoids the user wondering what's going on.
const statsReportLimit = 8 * time.Second

// report prints statistics if some number of blocks have been processed
// or more than a few seconds have passed since the last message.
func (st *insertStats) report(chain []*types.Block, index int, cache common.StorageSize) {
	// Fetch the timings for the batch
	var (
		now     = mclock.Now()
		elapsed = time.Duration(now) - time.Duration(st.startTime)
	)
	// If we're at the last block of the batch or report period reached, log
	if index == len(chain)-1 || elapsed >= statsReportLimit {
		var (
			end = chain[index]
			txs = countTransactions(chain[st.lastIndex : index+1])
		)
		context := []interface{}{
			"blocks", st.processed, "txs", txs, "mgas", float64(st.usedGas) / 1000000,
			"elapsed", common.PrettyDuration(elapsed), "mgasps", float64(st.usedGas) * 1000 / float64(elapsed),
			"number", end.Number(), "hash", end.Hash(), "cache", cache,
		}
		if st.queued > 0 {
			context = append(context, []interface{}{"queued", st.queued}...)
		}
		if st.ignored > 0 {
			context = append(context, []interface{}{"ignored", st.ignored}...)
		}
		// @TODO (rgeraldes)
		//log.Info("Imported new chain segment", context...)

		*st = insertStats{startTime: now, lastIndex: index + 1}
	}
}
*/

func countTransactions(chain []*types.Block) (c int) {
	for _, b := range chain {
		c += len(b.Transactions())
	}
	return c
}

// PostChainEvents iterates over the events generated by a chain insertion and
// posts them into the event feed.
// TODO: Should not expose PostChainEvents. The chain events should be posted in WriteBlock.
func (bc *BlockChain) PostChainEvents(events []interface{}, logs []*transaction.Log) {
	// post event logs for further processing
	if logs != nil {
		bc.logsFeed.Send(logs)
	}
	for _, event := range events {
		switch ev := event.(type) {
		case ChainEvent:
			bc.chainFeed.Send(ev)

		case ChainHeadEvent:
			bc.chainHeadFeed.Send(ev)
		}
	}
}

func (bc *BlockChain) update() {
	futureTimer := time.NewTicker(5 * time.Second)
	defer futureTimer.Stop()
	for {
		select {
		case <-futureTimer.C:
			bc.procFutureBlocks()
		case <-bc.doneCh:
			return
		}
	}
}

// BadBlocks returns a list of the last 'bad blocks' that the client has seen on the network
func (bc *BlockChain) BadBlocks() []*types.Block {
	blocks := make([]*types.Block, 0, bc.badBlocks.Len())
	for _, hash := range bc.badBlocks.Keys() {
		if blk, exist := bc.badBlocks.Peek(hash); exist {
			block := blk.(*types.Block)
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// addBadBlock adds a bad block to the bad-block LRU cache
func (bc *BlockChain) addBadBlock(block *types.Block) {
	bc.badBlocks.Add(block.Hash(), block)
}

// reportBlock logs a bad block error.
func (bc *BlockChain) reportBlock(block *types.Block, receipts transaction.Receipts, err error) {
	bc.addBadBlock(block)

	var receiptString string
	for _, receipt := range receipts {
		receiptString += fmt.Sprintf("\n%v\n", receipt.String())
	}
	if len(receiptString) == 0 {
		receiptString = "[EMPTY RECEIPTS]"
	}

	log.Error(fmt.Sprintf(`
########## BAD BLOCK #########
Number: %v
Hash: 0x%x
%v

Error: %v
##############################
`, block.Number(), block.Hash(), receiptString, err))
}

// InsertHeaderChain attempts to insert the given header chain in to the local
// chain, possibly creating a reorg. If an error is returned, it will return the
// index number of the failing header as well an error describing what went wrong.
//
// The verify parameter can be used to fine tune whether nonce verification
// should be done or not. The reason behind the optional check is because some
// of the header retrieval mechanisms already need to verify nonces, as well as
// because nonces can be verified sparsely, not needing to check each.
func (bc *BlockChain) InsertHeaderChain(chain []*types.Header, checkFreq int) (int, error) {
	start := time.Now()
	if i, err := bc.hc.ValidateHeaderChain(chain, checkFreq); err != nil {
		return i, err
	}

	// Make sure only one thread manipulates the chain at once
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	bc.wg.Add(1)
	defer bc.wg.Done()

	whFunc := func(header *types.Header) error {
		bc.mu.Lock()
		defer bc.mu.Unlock()

		return bc.hc.WriteHeader(header)
	}

	return bc.hc.InsertHeaderChain(chain, whFunc, start)
}

// writeHeader writes a header into the local chain, given that its parent is
// already known.
//
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (bc *BlockChain) writeHeader(header *types.Header) error {
	bc.wg.Add(1)
	bc.mu.Lock()

	err := bc.hc.WriteHeader(header)

	bc.mu.Unlock()
	bc.wg.Done()

	return err
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (bc *BlockChain) CurrentHeader() *types.Header {
	return bc.hc.CurrentHeader()
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (bc *BlockChain) GetHeader(hash crypto.Hash, number uint64) *types.Header {
	return bc.hc.GetHeader(hash, number)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (bc *BlockChain) GetHeaderByHash(hash crypto.Hash) *types.Header {
	return bc.hc.GetHeaderByHash(hash)
}

// HasHeader checks if a block header is present in the database or not, caching
// it if present.
func (bc *BlockChain) HasHeader(hash crypto.Hash, number uint64) bool {
	return bc.hc.HasHeader(hash, number)
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (bc *BlockChain) GetBlockHashesFromHash(hash crypto.Hash, max uint64) []crypto.Hash {
	return bc.hc.GetBlockHashesFromHash(hash, max)
}

// GetAncestor retrieves the Nth ancestor of a given block. It assumes that either the given block or
// a close ancestor of it is canonical. maxNonCanonical points to a downwards counter limiting the
// number of blocks to be individually checked before we reach the canonical chain.
//
// Note: ancestor == 0 returns the same block, 1 returns its parent and so on.
func (bc *BlockChain) GetAncestor(hash crypto.Hash, number, ancestor uint64, maxNonCanonical *uint64) (crypto.Hash, uint64) {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	return bc.hc.GetAncestor(hash, number, ancestor, maxNonCanonical)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (bc *BlockChain) GetHeaderByNumber(number uint64) *types.Header {
	return bc.hc.GetHeaderByNumber(number)
}

// Engine retrieves the blockchain's consensus engine.
func (bc *BlockChain) Engine() consensus.Engine { return bc.engine }

// SubscribeRemovedLogsEvent registers a subscription of RemovedLogsEvent.
func (bc *BlockChain) SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription {
	return bc.scope.Track(bc.rmLogsFeed.Subscribe(ch))
}

// SubscribeChainEvent registers a subscription of ChainEvent.
func (bc *BlockChain) SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription {
	return bc.scope.Track(bc.chainFeed.Subscribe(ch))
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (bc *BlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}

// SubscribeLogsEvent registers a subscription of []*transaction.Log.
func (bc *BlockChain) SubscribeLogsEvent(ch chan<- []*transaction.Log) event.Subscription {
	return bc.scope.Track(bc.logsFeed.Subscribe(ch))
}
