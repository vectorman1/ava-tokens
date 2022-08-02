package tokens_vm

import (
	"fmt"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ethereum/go-ethereum/log"
	"github.com/gorilla/rpc/v2"
	"github.com/pkg/errors"
	"time"
)

const (
	dataLen = 32
	Name    = "tokens-vm"
)

var (
	_ block.ChainVM = &VM{}

	Version = version.Current
)

type VM struct {
	// The context of this vm
	ctx       *snow.Context
	dbManager manager.Manager

	// State of this VM
	state State

	// ID of the preferred block
	preferred ids.ID

	// channel to send messages to the consensus engine
	toEngine chan<- common.Message

	// Proposed pieces of data that haven't been put into a block and proposed yet
	mempool [][dataLen]byte

	// Block ID --> Block
	// Each element is a block that passed verification but
	// hasn't yet been accepted/rejected
	verifiedBlocks map[ids.ID]*Block

	// Indicates that this VM has finised bootstrapping for the chain
	bootstrapped utils.AtomicBool
}

func (vm *VM) Initialize(
	ctx *snow.Context,
	dbManager manager.Manager,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	fxs []*common.Fx,
	appSender common.AppSender) error {
	version, err := vm.Version()
	if err != nil {
		return errors.Wrap(err, "failed to initialize vm")
	}
	log.Info("Initializing Timestamp VM", "Version", version)

	vm.dbManager = dbManager
	vm.ctx = ctx
	vm.toEngine = toEngine
	vm.verifiedBlocks = make(map[ids.ID]*Block)

	// Create new state
	vm.state = NewState(vm.dbManager.Current().Database, vm)

	// Initialize genesis
	if err := vm.initGenesis(genesisBytes); err != nil {
		return err
	}

	// Get last accepted
	lastAccepted, err := vm.state.GetLastAccepted()
	if err != nil {
		return err
	}

	ctx.Log.Info("initializing last accepted block as %s", lastAccepted)

	// Build off the most recently accepted block
	return vm.SetPreference(lastAccepted)
}

func (vm *VM) AppGossip(nodeID ids.NodeID, msg []byte) error {
	return nil
}

func (vm *VM) AppRequest(nodeID ids.NodeID, requestID uint32, time time.Time, request []byte) error {
	return errors.New("not implemented")
}

func (vm *VM) AppResponse(nodeID ids.NodeID, requestID uint32, response []byte) error {
	return errors.New("not implemented")
}

func (vm *VM) AppRequestFailed(nodeID ids.NodeID, requestID uint32) error {
	return errors.New("not implemented")
}

func (vm *VM) HealthCheck() (interface{}, error) {
	return nil, nil
}

func (vm *VM) Connected(id ids.NodeID, nodeVersion *version.Application) error {
	return nil
}

func (vm *VM) Disconnected(id ids.NodeID) error {
	return nil
}

// SetState sets this VM state according to given snow.State
func (vm *VM) SetState(state snow.State) error {
	switch state {
	// Engine reports it's bootstrapping
	case snow.Bootstrapping:
		return vm.onBootstrapStarted()
	case snow.NormalOp:
		// Engine reports it can start normal operations
		return vm.onNormalOperationsStarted()
	default:
		return snow.ErrUnknownState
	}
}

func (vm *VM) Shutdown() error {
	if vm.state == nil {
		return nil
	}

	return vm.state.Close() // close versionDB
}

func (vm *VM) Version() (string, error) {
	return Version.String(), nil
}

func (vm *VM) CreateStaticHandlers() (map[string]*common.HTTPHandler, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json.NewCodec(), "application/json")
	server.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	if err := server.RegisterService(&StaticService{}, Name); err != nil {
		return nil, err
	}

	return map[string]*common.HTTPHandler{
		"": {
			LockOptions: common.NoLock,
			Handler:     server,
		},
	}, nil
}

func (vm *VM) CreateHandlers() (map[string]*common.HTTPHandler, error) {
	server := rpc.NewServer()

	server.RegisterCodec(json.NewCodec(), "application/json")
	server.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")

	if err := server.RegisterService(&Service{vm: vm}, Name); err != nil {
		return nil, err
	}

	return map[string]*common.HTTPHandler{
		"": {
			Handler: server,
		},
	}, nil
}
func (vm *VM) GetBlock(id ids.ID) (snowman.Block, error) {
	return vm.getBlock(id)
}

func (vm *VM) ParseBlock(bytes []byte) (snowman.Block, error) {
	// A new empty block
	block := new(Block)

	// Unmarshal the byte repr. of the block into our empty block
	_, err := Codec.Unmarshal(bytes, block)
	if err != nil {
		return nil, err
	}

	// Initialize the block
	block.Initialize(bytes, choices.Processing, vm)

	if blk, err := vm.getBlock(block.ID()); err == nil {
		// If we have seen this block before, return it with the most up-to-date
		// info
		return blk, nil
	}

	// Return the block
	return block, nil
}

func (vm *VM) BuildBlock() (snowman.Block, error) {
	if len(vm.mempool) == 0 { // There is no block to be built
		return nil, errors.New("no pending blocks")
	}

	// Get the value to put in the new block
	value := vm.mempool[0]
	vm.mempool = vm.mempool[1:]

	// Notify consensus engine that there are more pending data for blocks
	// (if that is the case) when done building this block
	if len(vm.mempool) > 0 {
		defer vm.NotifyBlockReady()
	}

	// Gets Preferred Block
	preferredBlock, err := vm.getBlock(vm.preferred)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get preferred block")
	}
	preferredHeight := preferredBlock.Height()

	// Build the block with preferred height
	newBlock, err := vm.NewBlock(vm.preferred, preferredHeight+1, value, time.Now())
	if err != nil {
		return nil, fmt.Errorf("couldn't build block: %w", err)
	}

	// Verifies block
	if err := newBlock.Verify(); err != nil {
		return nil, err
	}
	return newBlock, nil
}

func (vm *VM) SetPreference(id ids.ID) error {
	vm.preferred = id
	return nil
}

func (vm *VM) LastAccepted() (ids.ID, error) {
	return vm.state.GetLastAccepted()
}

// NotifyBlockReady tells the consensus engine that a new block
// is ready to be created
func (vm *VM) NotifyBlockReady() {
	select {
	case vm.toEngine <- common.PendingTxs:
	default:
		vm.ctx.Log.Debug("dropping message to consensus engine")
	}
}

// NewBlock returns a new Block where:
// - the block's parent is [parentID]
// - the block's data is [data]
// - the block's timestamp is [timestamp]
func (vm *VM) NewBlock(parentID ids.ID, height uint64, data [dataLen]byte, timestamp time.Time) (*Block, error) {
	block := new(Block)

	// Initialize the block

	// Get the byte representation of the block
	blockBytes, err := Codec.Marshal(CodecVersion, block)
	if err != nil {
		return nil, err
	}

	// Initialize the block by providing it with its byte representation
	// and a reference to this VM
	block.Initialize(blockBytes, choices.Processing, vm)
	return block, nil
}

func (vm *VM) initGenesis(genesisData []byte) error {
	stateInitialized, err := vm.state.IsInitialized()
	if err != nil {
		return err
	}

	// if state is already initialized, skip init genesis.
	if stateInitialized {
		return nil
	}

	if len(genesisData) > dataLen {
		return errors.New("bad genesis bytes data")
	}

	// genesisData is a byte slice but each block contains an byte array
	// Take the first [dataLen] bytes from genesisData and put them in an array
	var genesisDataArr [dataLen]byte
	copy(genesisDataArr[:], genesisData)

	// Create the genesis block
	// Timestamp of genesis block is 0. It has no parent.
	genesisBlock, err := vm.NewBlock(ids.Empty, 0, genesisDataArr, time.Unix(0, 0))
	if err != nil {
		log.Error("error while creating genesis block: %v", err)
		return err
	}

	// Put genesis block to state
	if err := vm.state.PutBlock(genesisBlock); err != nil {
		log.Error("error while saving genesis block: %v", err)
		return err
	}

	// Accept the genesis block
	// Sets [vm.lastAccepted] and [vm.preferred]
	if err := genesisBlock.Accept(); err != nil {
		return fmt.Errorf("error accepting genesis block: %w", err)
	}

	// Mark this vm's state as initialized, so we can skip initGenesis in further restarts
	if err := vm.state.SetInitialized(); err != nil {
		return fmt.Errorf("error while setting db to initialized: %w", err)
	}

	// Flush VM's database to underlying db
	return vm.state.Commit()
}

func (vm *VM) getBlock(blkID ids.ID) (*Block, error) {
	// If block is in memory, return it.
	if blk, exists := vm.verifiedBlocks[blkID]; exists {
		return blk, nil
	}

	return vm.state.GetBlock(blkID)
}

// onBootstrapStarted marks this VM as bootstrapping
func (vm *VM) onBootstrapStarted() error {
	vm.bootstrapped.SetValue(false)
	return nil
}

// onNormalOperationsStarted marks this VM as bootstrapped
func (vm *VM) onNormalOperationsStarted() error {
	// No need to set it again
	if vm.bootstrapped.GetValue() {
		return nil
	}
	vm.bootstrapped.SetValue(true)
	return nil
}
