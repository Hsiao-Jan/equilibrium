// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/node/services/archive/types"
	"github.com/kowala-tech/equilibrium/state/accounts/transaction"
	"github.com/kowala-tech/kcoin/client/rlp"
	"go.uber.org/zap"
)

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func ReadTxLookupEntry(db DatabaseReader, hash crypto.Hash) (crypto.Hash, uint64, uint64) {
	data, _ := db.Get(txLookupKey(hash))
	if len(data) == 0 {
		return crypto.Hash{}, 0, 0
	}
	var entry TxLookupEntry
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		log.Error("Invalid transaction lookup entry RLP", zap.String("hash", hash.String()), zap.Error(err))
		return crypto.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func WriteTxLookupEntries(db DatabaseWriter, block *types.Block) {
	for i, tx := range block.Transactions() {
		entry := TxLookupEntry{
			BlockHash:  block.Hash(),
			BlockIndex: block.NumberU64(),
			Index:      uint64(i),
		}
		data, err := rlp.EncodeToBytes(entry)
		if err != nil {
			log.Fatal("Failed to encode transaction lookup entry", zap.Error(err))
		}
		if err := db.Put(txLookupKey(tx.Hash()), data); err != nil {
			log.Fatal("Failed to store transaction lookup entry", zap.Error(err))
		}
	}
}

// DeleteTxLookupEntry removes all transaction data associated with a hash.
func DeleteTxLookupEntry(db DatabaseDeleter, hash crypto.Hash) {
	db.Delete(txLookupKey(hash))
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadTransaction(db DatabaseReader, hash crypto.Hash) (*transaction.Transaction, crypto.Hash, uint64, uint64) {
	blockHash, blockNumber, txIndex := ReadTxLookupEntry(db, hash)
	if blockHash == (crypto.Hash{}) {
		return nil, crypto.Hash{}, 0, 0
	}
	body := ReadBody(db, blockHash, blockNumber)
	if body == nil || len(body.Transactions) <= int(txIndex) {
		log.Error("Transaction referenced missing", zap.Uint64("number", blockNumber), zap.String("hash", blockHash.String()), zap.Uint64("index", txIndex))
		return nil, crypto.Hash{}, 0, 0
	}
	return body.Transactions[txIndex], blockHash, blockNumber, txIndex
}

// ReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func ReadReceipt(db DatabaseReader, hash crypto.Hash) (*transaction.Receipt, crypto.Hash, uint64, uint64) {
	blockHash, blockNumber, receiptIndex := ReadTxLookupEntry(db, hash)
	if blockHash == (crypto.Hash{}) {
		return nil, crypto.Hash{}, 0, 0
	}
	receipts := ReadReceipts(db, blockHash, blockNumber)
	if len(receipts) <= int(receiptIndex) {
		log.Error("Receipt refereced missing", zap.Uint64("number", blockNumber), zap.String("hash", blockHash.String()), zap.Uint64("index", receiptIndex))
		return nil, crypto.Hash{}, 0, 0
	}
	return receipts[receiptIndex], blockHash, blockNumber, receiptIndex
}

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func ReadBloomBits(db DatabaseReader, bit uint, section uint64, head crypto.Hash) ([]byte, error) {
	return db.Get(bloomBitsKey(bit, section, head))
}

// WriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func WriteBloomBits(db DatabaseWriter, bit uint, section uint64, head crypto.Hash, bits []byte) {
	if err := db.Put(bloomBitsKey(bit, section, head), bits); err != nil {
		log.Fatal("Failed to store bloom bits", zap.Error(err))
	}
}
