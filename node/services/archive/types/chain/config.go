package chain

import (
	"math/big"

	"github.com/kowala-tech/equilibrium/common/hexutil"
	"github.com/kowala-tech/equilibrium/crypto"
)

//go:generate gencodec -type Config -field-override configMarshaling -out gen_config_json.go

// Config represents the chain configuration.
type Config struct {
	ChainID *big.Int `json:"chainID" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *crypto.Hash `json:"hash" rlp:"-"`
}

type configMarshaling struct {
	ChainID *hexutil.Big
}
