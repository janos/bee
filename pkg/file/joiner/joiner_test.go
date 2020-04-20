// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package joiner_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/ethersphere/bee/pkg/file/joiner"
	filetest "github.com/ethersphere/bee/pkg/file/testing"
	"github.com/ethersphere/bee/pkg/storage"
	"github.com/ethersphere/bee/pkg/storage/mock"
	"github.com/ethersphere/bee/pkg/swarm"
)

// TestJoiner verifies that a newly created joiner
// returns the data stored in the store for a given reference
func TestJoiner(t *testing.T) {
	store := mock.NewStorer()

	joiner := joiner.NewSimpleJoiner(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	_, _, err = joiner.Join(ctx, swarm.ZeroAddress)
	if err != storage.ErrNotFound {
		t.Fatalf("expected ErrNotFound for %x", swarm.ZeroAddress)
	}

	mockAddrHex := fmt.Sprintf("%064s", "2a")
	mockAddr := swarm.MustParseHexAddress(mockAddrHex)
	mockData := []byte("foo")
	mockDataLengthBytes := make([]byte, 8)
	mockDataLengthBytes[0] = 0x03;
	mockChunk := swarm.NewChunk(mockAddr, append(mockDataLengthBytes, mockData...))
	_, err = store.Put(ctx, storage.ModePutUpload, mockChunk)
	if err != nil {
		t.Fatal(err)
	}

	joinReader, l, err := joiner.Join(ctx, mockAddr)
	if l != int64(len(mockData)) {
		t.Fatalf("expected join data length %d, got %d", len(mockData), l)
	}
	joinData, err := ioutil.ReadAll(joinReader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(joinData, mockData) {
		t.Fatalf("retrieved data '%x' not like original data '%x'", joinData, mockData)
	}
}

// TestJoinerWithReference verifies that a chunk reference is correctly resolved
// and the underlying data is returned
func TestJoinerWithReference(t *testing.T) {
	store := mock.NewStorer()
	joiner := joiner.NewSimpleJoiner(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	_, _, err = joiner.Join(ctx, swarm.ZeroAddress)
	if err != storage.ErrNotFound {
		t.Fatalf("expected ErrNotFound for %x", swarm.ZeroAddress)
	}

	rootChunk := filetest.GenerateTestRandomFileChunk(swarm.ZeroAddress, swarm.ChunkSize*2, swarm.SectionSize*2)

	firstAddress := swarm.NewAddress(rootChunk.Data()[:swarm.SectionSize])
	firstChunk := filetest.GenerateTestRandomFileChunk(firstAddress, swarm.ChunkSize, swarm.ChunkSize)
	_, err = store.Put(ctx, storage.ModePutUpload, firstChunk)
	if err != nil {
		t.Fatal(err)
	}

	secondAddress := swarm.NewAddress(rootChunk.Data()[swarm.SectionSize:])
	secondChunk := filetest.GenerateTestRandomFileChunk(secondAddress, swarm.ChunkSize, swarm.ChunkSize)
	_, err = store.Put(ctx, storage.ModePutUpload, secondChunk)
	if err != nil {
		t.Fatal(err)
	}

	joinReader, l, err := joiner.Join(ctx, rootChunk.Address())
	if l != int64(len(firstChunk.Data()) + len(secondChunk.Data())) {
		t.Fatalf("expected join data length %d, got %d", len(firstChunk.Data()) + len(secondChunk.Data()), l)
	}

	resultBuffer := make([]byte, 3)
	n, err := joinReader.Read(resultBuffer)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(resultBuffer) {
		t.Fatalf("expected read count %d, got %d", len(resultBuffer), n)
	}
	if !bytes.Equal(resultBuffer, firstChunk.Data()[:len(resultBuffer)]) {
		t.Fatalf("expected resultbuffer %v, got %v", resultBuffer, firstChunk.Data()[:len(resultBuffer)])
	}
}
