// Copyright 2019 The go-ethereum Authors
// This file is part of the CortexFoundation library.
//
// The CortexFoundation library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The CortexFoundation library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the CortexFoundation library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"errors"
	"fmt"
	"sync"

	"github.com/holiman/uint256"
)

var memoryPool = sync.Pool{
	New: func() any {
		return &Memory{}
	},
}

// Memory implements a simple memory model for the cortex virtual machine.
type Memory struct {
	store       []byte
	lastGasCost uint64
}

// NewMemory returns a new memory memory model.
func NewMemory() *Memory {
	return memoryPool.Get().(*Memory)
}

// Free returns the memory to the pool.
func (m *Memory) Free() {
	// To reduce peak allocation, return only smaller memory instances to the pool.
	const maxBufferSize = 16 << 10
	if cap(m.store) <= maxBufferSize {
		m.store = m.store[:0]
		m.lastGasCost = 0
		memoryPool.Put(m)
	}
}

// Set sets offset + size to value
func (m *Memory) Set(offset, size uint64, value []byte) {
	// It's possible the offset is greater than 0 and size equals 0. This is because
	// the calcMemSize (common.go) could potentially return 0 when size is zero (NO-OP)
	if size > 0 {
		// length of store may never be less than offset + size.
		// The store should be resized PRIOR to setting the memory
		if offset+size > uint64(len(m.store)) {
			panic("invalid memory: store empty")
		}
		copy(m.store[offset:offset+size], value)
	}
}

// Set32 sets the 32 bytes starting at offset to the value of val, left-padded with zeroes to
// 32 bytes.
func (m *Memory) Set32(offset uint64, val *uint256.Int) {
	// length of store may never be less than offset + size.
	// The store should be resized PRIOR to setting the memory
	if offset+32 > uint64(len(m.store)) {
		panic("invalid memory: store empty")
	}
	// Fill in relevant bits
	val.PutUint256(m.store[offset:])
}

// Resize resizes the memory to size
func (m *Memory) Resize(size uint64) {
	if uint64(m.Len()) < size {
		m.store = append(m.store, make([]byte, size-uint64(m.Len()))...)
	}
}

// GetCopy returns offset + size as a new slice
func (m *Memory) GetCopy(offset, size uint64) (cpy []byte) {
	if size == 0 {
		return nil
	}

	// memory is always resized before being accessed, no need to check bounds
	cpy = make([]byte, size)
	copy(cpy, m.store[offset:offset+size])
	return
}

// GetPtr returns the offset + size
func (m *Memory) GetPtr(offset, size uint64) []byte {
	if size == 0 {
		return nil
	}

	// memory is always resized before being accessed, no need to check bounds
	return m.store[offset : offset+size]
}

// Len returns the length of the backing slice
func (m *Memory) Len() int {
	return len(m.store)
}

// Data returns the backing slice
func (m *Memory) Data() []byte {
	return m.store
}

// Print dumps the content of the memory.
func (m *Memory) Print() {
	fmt.Printf("### mem %d bytes ###\n", len(m.store))
	if len(m.store) > 0 {
		addr := 0
		for i := 0; i+32 <= len(m.store); i += 32 {
			fmt.Printf("%03d: % x\n", addr, m.store[i:i+32])
			addr++
		}
	} else {
		fmt.Println("-- empty --")
	}
	fmt.Println("####################")
}

func (m *Memory) GetSolidityBytes(slot int64) ([]byte, error) {
	bigLen := new(uint256.Int)
	length_buff := m.GetPtr(uint64(slot), 32)
	bigLen.SetBytes(length_buff)
	buff := m.GetPtr(uint64(slot+32), bigLen.Uint64())
	return buff, nil
}

/*func (m* Memory) GetLengthOfSolidityBytes(slot int64) (uint64, error) {
       bigLen := big.NewInt(0)
       length_buff := m.GetPtr(slot, 32)
       bigLen.SetBytes(length_buff)
       return bigLen.Uint64(), nil
}

func (m* Memory) GetLengthOfSolidityUint256Array(slot int64) (uint64, error) {
       bigLen := big.NewInt(0)
       length_buff := m.GetPtr(slot, 32)
       bigLen.SetBytes(length_buff)
       return bigLen.Uint64(), nil
}*/

func (m *Memory) WriteSolidityUint256Array(slot int64, data []byte) error {
	bigLen := new(uint256.Int)
	length_buff := m.GetPtr(uint64(slot), 32)
	bigLen.SetBytes(length_buff)
	// uint256 has 32 bytes
	bigLen.Mul(bigLen, uint256.NewInt(32))
	if int64(len(data)) > int64(bigLen.Uint64()) {
		return errors.New(fmt.Sprintf("solidity memory bytes length not match %d != %d", len(data), bigLen.Uint64()))
	}
	m.Set(uint64(slot+32), bigLen.Uint64(), data)
	return nil
}

func (m *Memory) GetSolidityUint256(slot int64) ([]byte, error) {
	bigLen := new(uint256.Int)
	length_buff := m.GetPtr(uint64(slot), 32)
	bigLen.SetBytes(length_buff)
	bigLen.Mul(bigLen, uint256.NewInt(32))
	buff := m.GetPtr(uint64(slot+32), bigLen.Uint64())
	return buff, nil
}

// Copy copies data from the src position slice into the dst position.
// The source and destination may overlap.
// OBS: This operation assumes that any necessary memory expansion has already been performed,
// and this method may panic otherwise.
func (m *Memory) Copy(dst, src, len uint64) {
	if len == 0 {
		return
	}
	copy(m.store[dst:], m.store[src:src+len])
}
