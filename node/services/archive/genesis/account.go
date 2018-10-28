package genesis

import (
	"math/big"

	"github.com/kowala-tech/equilibrium/crypto"
	"github.com/kowala-tech/equilibrium/state/accounts"
)

// @TODO (rgeraldes) - balance required? default balance to 0

// Accounts specifies the initial state that is part of the genesis block.
type Accounts map[accounts.Address]Account

// Account represents a genesis account.
type Account struct {
	Code    []byte                      `json:"code,omitempty"`
	Storage map[crypto.Hash]crypto.Hash `json:"storage,omitempty"`
	Balance *big.Int                    `json:"balance" gencodec:"required"`
}
