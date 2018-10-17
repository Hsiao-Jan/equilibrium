package params

import "math/big"

var (
	OracleEpochDuration = new(big.Int).SetUint64(900)
	OracleUpdatePeriod  = new(big.Int).SetUint64(15)
)
