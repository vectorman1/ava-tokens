/*
 * Copyright 2022 LimeChain Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tokens_vm

import (
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/pkg/errors"
	"time"
)

var _ snowman.Block = &Block{}

type Block struct {
	ParentID ids.ID `serialize:"true" json:"parentID"`

	id        ids.ID
	bytes     []byte
	height    uint64
	status    choices.Status
	timestamp time.Time
	vm        *VM
}

func (b *Block) Initialize(bytes []byte, status choices.Status, vm *VM) {
	b.bytes = bytes
	b.id = hashing.ComputeHash256Array(b.bytes)
	b.status = status
	b.vm = vm
}

func (b *Block) ID() ids.ID {
	return b.id
}

func (b *Block) Accept() error {
	b.status = choices.Accepted // Change state of this block
	blkID := b.ID()

	// Persist data
	if err := b.vm.state.PutBlock(b); err != nil {
		return err
	}

	// Set last accepted ID to this block ID
	if err := b.vm.state.SetLastAccepted(blkID); err != nil {
		return err
	}

	// Delete this block from verified blocks as it's accepted
	delete(b.vm.verifiedBlocks, b.ID())

	// Commit changes to database
	return b.vm.state.Commit()
}

func (b *Block) Reject() error {
	b.status = choices.Rejected // Change state of this block
	if err := b.vm.state.PutBlock(b); err != nil {
		return err
	}
	// Delete this block from verified blocks as it's rejected
	delete(b.vm.verifiedBlocks, b.ID())
	// Commit changes to database
	return b.vm.state.Commit()
}

func (b *Block) Status() choices.Status {
	return b.status
}

func (b *Block) Parent() ids.ID {
	return b.ParentID
}

func (b *Block) Verify() error {
	// Get [b]'s parent
	parentID := b.Parent()
	parent, err := b.vm.GetBlock(parentID)
	if err != nil {
		return errors.New("could not get parent block")
	}

	// Ensure [b]'s height comes right after its parent's height
	if expectedHeight := parent.Height() + 1; expectedHeight != b.Height() {
		return fmt.Errorf(
			"expected block to have height %d, but found %d",
			expectedHeight,
			b.Height(),
		)
	}

	// Ensure [b]'s timestamp is not more than an hour
	// ahead of this node's time
	if b.Timestamp().Unix() >= time.Now().Add(time.Hour).Unix() {
		return errors.New("timestamp is too late")
	}

	// Put that block to verified blocks in memory
	b.vm.verifiedBlocks[b.ID()] = b

	return nil
}

func (b *Block) Bytes() []byte {
	return b.bytes
}

func (b *Block) Height() uint64 {
	return b.height
}

func (b *Block) Timestamp() time.Time {
	return b.timestamp
}
