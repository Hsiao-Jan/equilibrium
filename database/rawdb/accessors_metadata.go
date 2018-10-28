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
	"encoding/json"

	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/encoding/rlp"
	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/network"
	"go.uber.org/zap"
)

// ReadDatabaseVersion retrieves the version number of the database.
func ReadDatabaseVersion(db DatabaseReader) int {
	var version int

	enc, _ := db.Get(databaseVerisionKey)
	rlp.DecodeBytes(enc, &version)

	return version
}

// WriteDatabaseVersion stores the version number of the database
func WriteDatabaseVersion(db DatabaseWriter, version int) {
	enc, _ := rlp.EncodeToBytes(version)
	if err := db.Put(databaseVerisionKey, enc); err != nil {
		log.Fatal("Failed to store the database version", zap.Error(err))
	}
}

// ReadNetworkSettings retrieves the network settings based on the given genesis hash.
func ReadNetworkSettings(db DatabaseReader, hash crypto.Hash) *network.Settings {
	data, _ := db.Get(configKey(hash))
	if len(data) == 0 {
		return nil
	}
	var config network.Settings
	if err := json.Unmarshal(data, &config); err != nil {
		log.Error("Invalid chain config JSON", zap.String("hash", hash.String()), zap.Error(err))
		return nil
	}
	return &config
}

// WriteNetworkSettings writes the network settings to the database.
func WriteNetworkSettings(db DatabaseWriter, hash crypto.Hash, cfg *network.Settings) {
	if cfg == nil {
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		log.Fatal("Failed to JSON encode chain config", zap.Error(err))
	}
	if err := db.Put(configKey(hash), data); err != nil {
		log.Fatal("Failed to store chain config", zap.Error(err))
	}
}

// ReadPreimage retrieves a single preimage of the provided hash.
func ReadPreimage(db DatabaseReader, hash crypto.Hash) []byte {
	data, _ := db.Get(preimageKey(hash))
	return data
}

// WritePreimages writes the provided set of preimages to the database. `number` is the
// current block number, and is used for debug messages only.
func WritePreimages(db DatabaseWriter, number uint64, preimages map[crypto.Hash][]byte) {
	for hash, preimage := range preimages {
		if err := db.Put(preimageKey(hash), preimage); err != nil {
			log.Fatal("Failed to store trie preimage", zap.Error(err))
		}
	}
	preimageCounter.Inc(int64(len(preimages)))
	preimageHitCounter.Inc(int64(len(preimages)))
}
