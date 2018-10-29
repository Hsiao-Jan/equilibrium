package chain

import (
	"time"
)

// CacheConfig contains the configuration values for the trie caching/pruning
// that's resident in a blockchain.
type CacheConfig struct {
	Disabled      bool          // Whether to disable trie write caching (archive node).
	TrieNodeLimit int           // Memory limit (MB) at which to flush the current in-memory trie to disk.
	TrieTimeLimit time.Duration // Time limit after which to flush the current in-memory trie to disk.
}
