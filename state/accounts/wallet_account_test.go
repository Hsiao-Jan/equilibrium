package accounts

import (
	"testing"

	"github.com/kowala-tech/equilibrium/types"
	"github.com/stretchr/testify/assert"
)

func TestNewWalletAccount(t *testing.T) {
	address := types.Address{1}
	account := Account{Address: address}
	wallet := &MockWallet{}
	wallet.On("Contains", account).Return(true)

	walletAccount, err := NewWalletAccount(wallet, account)
	assert.NoError(t, err)

	assert.Equal(t, account, walletAccount.account)
}

func TestNewWalletAccountFailsIfAddressDoesntExistInWallet(t *testing.T) {
	address := types.Address{1}
	account := Account{Address: address}
	wallet := &MockWallet{}
	wallet.On("Contains", account).Return(false)

	_, err := NewWalletAccount(wallet, account)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidAccountAddress{account}, err)
}
