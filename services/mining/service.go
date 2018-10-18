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

package mining

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/kowala-tech/core_contracts/consensus"
	"github.com/kowala-tech/equilibrium/log"
	"github.com/kowala-tech/equilibrium/services/archive"
	"github.com/kowala-tech/equilibrium/services/mining/validator"
	"github.com/kowala-tech/kcoin/client/accounts"
	"github.com/kowala-tech/kcoin/client/common"
	"github.com/kowala-tech/kcoin/client/p2p"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
)

type Service struct {
	serviceMu sync.RWMutex
	coinbase  types.Address
	deposit   *big.Int

	validator    validator.Validator
	validatorMgr *consensus.Consensus

	protocolMgr *ProtocolManager

	overlay *p2p.Host
}

func New(cfg *Config, archive *archive.Service) (*Service, error) {
	service := &Service{
		coinbase: cfg.Coinbase,
		deposit:  cfg.Deposit,
		gasPrice: cfg.GasPrice,
	}

	log.Info("Initialising Mining Service...")

	/*
		service.validator = validator.New(service.kowalaService, service.validatorMgr, service.globalEvents)
		service.validator.SetExtra(makeExtraData(config.ExtraData)

		if service.protocolMgr, err = NewProtocolManager(service.kowalaService.ChainID().Uint64(), service.validator, service.validator.VotingSystem(), service.kowalaService.BlockChain(), service.log); err != nil {
			return nil, err
		}
	*/

	return service, nil
}

func (s *Service) Deposit() *big.Int {
	s.lock.RLock()
	deposit := s.deposit
	s.lock.RUnlock()

	return deposit
}

// SetDeposit is an helper to set the deposit amount via console/cli flags.
func (s *Service) SetDeposit(deposit *big.Int) error {
	s.lock.Lock()
	s.deposit = deposit
	s.lock.Unlock()

	return s.validator.SetDeposit(deposit)
}

func (s *Service) Coinbase() (types.Address, error) {
	s.lock.RLock()
	coinbase := s.coinbase
	s.lock.RUnlock()

	if coinbase != (types.Address{}) {
		return coinbase, nil
	}
	if wallets := s.AccountMgr().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			return accounts[0].Address, nil
		}
	}
	return common.Address{}, fmt.Errorf("coinbase address must be explicitly specified")
}

// SetCoinbase is an helper to set the coinbase via console/cli flags.
func (s *Service) SetCoinbase(coinbase types.Address) {
	s.lock.Lock()
	s.coinbase = coinbase
	s.lock.Unlock()

	walletAccount, err := s.getWalletAccount()
	if err != nil {
		log.Error("error SetCoinbase on validator getWalletAccount", zap.Error(err))
	}

	if err := s.validator.SetCoinbase(walletAccount); err != nil {
		log.Error("error SetCoinbase on validator setCoinbase", zap.Error(err))
	}
}

// @TOOD (remove dependency to S)
func (s *Service) getWalletAccount() (accounts.WalletAccount, error) {
	account := accounts.Account{Address: s.coinbase}
	wallet, err := s.accountMgr.Find(account)
	if err != nil {
		return nil, err
	}
	return accounts.NewWalletAccount(wallet, account)
}

// GetMinimumDeposit returns the minimum amount required to join the mining pool.
func (s *Service) GetMinimumDeposit() (*big.Int, error) {
	return s.validatorMgr.MinimumDeposit()
}

func (s *Service) StartValidating() error {
	if err := s.protocolMgr.Start(s.overlay); err != nil {
		log.Error("Failed to start the protocol manager", zap.Error(err))
	}

	_, err := s.Coinbase()
	if err != nil {
		log.Error("Cannot start consensus validation without coinbase", zap.Error(err))
		return fmt.Errorf("coinbase missing: %v", err)
	}

	deposit := s.Deposit()

	// @NOTE (rgeraldes) - ignored transaction rejection mechanism introduced to speed sync times
	// @TODO (rgeraldes) - review (does it make sense to have a list of transactions before the election or not)
	// atomic.StoreUint32(&s.protocolMgr.acceptTxs, 1)

	walletAccount, err := s.getWalletAccount()
	if err != nil {
		return fmt.Errorf("error starting validating: %v", err)
	}

	s.validator.Start(walletAccount, deposit)
	return nil
}

func (s *Service) IsValidating() bool                 { return s.validator.Validating() }
func (s *Service) IsRunning() bool                    { return s.validator.Running() }
func (s *Service) Validator() validator.Validator     { return s.validator }
func (s *Service) ValidatorMgr() *consensus.Consensus { return s.validatorMgr }

func (s *Service) StopValidating() {
	// warning: order should not be changed
	if err := s.validator.Stop(); err != nil {
		log.Error("Error stopping Consensus", "err", err)
	}

	s.protocolMgr.Stop()
}

func (s *Service) Start(host *p2p.Host) error { return nil }

func (s *Service) Stop() error {
	if s.IsValidating() {
		s.StopValidating()
	}
}
