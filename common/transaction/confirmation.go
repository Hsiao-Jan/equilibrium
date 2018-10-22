// Copyright © 2018 Kowala SEZC <info@kowala.tech>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transaction

import (
	"context"
	"time"

	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/types"
)

type Backend interface {
	TransactionReceipt(ctx context.Context, txHash types.Hash) (*types.Receipt, error)
}

// WaitMinedWithTimeout waits for tx to be mined on the blockchain within a given period of time.
func WaitMinedWithTimeout(backend Backend, txHash types.Hash, duration time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	return WaitMined(ctx, backend, txHash)
}

// WaitMined waits for tx to be mined on the blockchain.
// It stops waiting when the context is canceled.
func WaitMined(ctx context.Context, backend Backend, txHash types.Hash) (*types.Receipt, error) {
	queryTicker := time.NewTicker(time.Second)
	defer queryTicker.Stop()

	for {
		receipt, err := backend.TransactionReceipt(ctx, txHash)
		if receipt != nil {
			return receipt, nil
		}
		if err != nil {
			return nil, err
		} else {
			log.Debug("Transaction not yet mined")
		}
		// Wait for the next round.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-queryTicker.C:
		}
	}
}
