package tokens_vm

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/prefixdb"
	"github.com/ava-labs/avalanchego/database/versiondb"
	"github.com/ava-labs/avalanchego/vms/components/avax"
)

var (
	singletonStatePrefix = []byte("singleton")
	blockStatePrefix     = []byte("block")

	_ State = &state{}
)

type State interface {
	avax.SingletonState
	BlockState

	Commit() error
	Close() error
}

type state struct {
	avax.SingletonState
	BlockState

	baseDB *versiondb.Database
}

func NewState(db database.Database, vm *VM) State {
	// create a new baseDB
	baseDB := versiondb.New(db)

	// create a prefixed "blockDB" from baseDB
	blockDB := prefixdb.New(blockStatePrefix, baseDB)
	// create a prefixed "singletonDB" from baseDB
	singletonDB := prefixdb.New(singletonStatePrefix, baseDB)

	// return state with created sub state components
	return &state{
		BlockState:     NewBlockState(blockDB, vm),
		SingletonState: avax.NewSingletonState(singletonDB),
		baseDB:         baseDB,
	}
}

func (s *state) Commit() error {
	return s.baseDB.Commit()
}

func (s *state) Close() error {
	//TODO implement me
	return s.baseDB.Close()
}
