package archive

import (
	"github.com/kowala-tech/equilibrium/database"
)

// OpenDB creates the chain database.
func OpenDB(dataDir string, name string, cache int, handles int) (database.Database, error) {
	if dataDir == "" {
		return database.NewMemDatabase(), nil
	}

	db, err := database.NewLDBDatabase(name, cache, handles)
	if err != nil {
		return nil, err
	}
	db.Meter("kcoin/db/chaindata/")

	return db, nil
}
