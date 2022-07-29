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
	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/pkg/errors"
)

const (
	lastAcceptedByte byte = iota
)

var _ BlockState = &blockState{}

var lastAcceptedKey = []byte{lastAcceptedByte}

type BlockState interface {
	GetBlock(blkID ids.ID) (*Block, error)
	PutBlock(blk *Block) error

	GetLastAccepted() (ids.ID, error)
	SetLastAccepted(ids.ID) error
}

type blockState struct {
	// cache to store blocks
	blkCache cache.Cacher
	// block database
	blockDB      database.Database
	lastAccepted ids.ID

	// vm reference
	vm *VM
}

type blkWrapper struct {
	Blk    []byte         `serialize:"true"`
	Status choices.Status `serialize:"true"`
}

func (s *blockState) GetBlock(blkID ids.ID) (*Block, error) {
	if blk, ok := s.blkCache.Get(blkID); ok {
		if blk == nil {
			return nil, database.ErrNotFound
		}
		return blk.(*Block), nil
	}

	// get block bytes from db with the blkID key
	wrappedBytes, err := s.blockDB.Get(blkID[:])
	if err != nil {
		// we could not find it in the db, let's cache this blkID with nil value
		// so next time we try to fetch the same key we can return error
		// without hitting the database
		if err == database.ErrNotFound {
			s.blkCache.Put(blkID, nil)
		}
		// could not find the block, return error
		return nil, errors.Wrap(err, "could not find block")
	}

	blkw := new(blkWrapper)
	if _, err := Codec.Unmarshal(wrappedBytes, blkw); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block wrapper")
	}

	blk := new(Block)
	if _, err := Codec.Unmarshal(blkw.Blk, blk); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}

	blk.Initialize(blkw.Blk, blkw.Status, s.vm)

	s.blkCache.Put(blkID, blk)

	return blk, nil
}

func (s *blockState) PutBlock(blk *Block) error {
	blkw := &blkWrapper{
		Blk:    blk.Bytes(),
		Status: blk.Status(),
	}

	wrappedBytes, err := Codec.Marshal(CodecVersion, blkw)
	if err != nil {
		return errors.Wrap(err, "could not marshal block")
	}

	blkID := blk.ID()

	// save to cache
	s.blkCache.Put(blkID, blk)

	// save to db
	err = s.blockDB.Put(blkID[:], wrappedBytes)
	if err != nil {
		return errors.Wrap(err, "could not save block")
	}

	return nil
}

func (s *blockState) GetLastAccepted() (ids.ID, error) {
	if s.lastAccepted != ids.Empty {
		return s.lastAccepted, nil
	}

	lastAcceptedBytes, err := s.blockDB.Get(lastAcceptedKey)
	if err != nil {
		return ids.ID{}, err
	}

	lastAccepted, err := ids.ToID(lastAcceptedBytes)
	if err != nil {
		return ids.ID{}, err
	}

	// Mutex?
	s.lastAccepted = lastAccepted
	return lastAccepted, nil
}

func (s *blockState) SetLastAccepted(id ids.ID) error {
	if s.lastAccepted == id {
		return nil
	}

	// mutex?
	s.lastAccepted = id

	err := s.blockDB.Put(lastAcceptedKey, id[:])
	if err != nil {
		return errors.Wrap(err, "could not save last accepted block")
	}

	return nil
}
