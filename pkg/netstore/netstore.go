// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethersphere/bee/pkg/logging"
	"github.com/ethersphere/bee/pkg/retrieval"
	"github.com/ethersphere/bee/pkg/storage"
	"github.com/ethersphere/bee/pkg/swarm"
)

type store struct {
	storage.Storer
	retrieval  retrieval.Interface
	validators []swarm.ChunkValidator
	logger     logging.Logger
	recoveryCallback func(ctx context.Context, chunkAddress swarm.Address) error // this is the callback to be executed when a chunk fails to be retrieved
}

// New returns a new NetStore that wraps a given Storer.
func New(s storage.Storer, r retrieval.Interface, logger logging.Logger, validators ...swarm.ChunkValidator) storage.Storer {
	return &store{Storer: s, retrieval: r, logger: logger, validators: validators}
}

// WithRecoveryCallback allows injecting a callback func on the NetStore struct
func (s *store) WithRecoveryCallback(f func(ctx context.Context, chunkAddress swarm.Address) error) *store {
	s.recoveryCallback = f
	return s
}

// Get retrieves a given chunk address.
// It will request a chunk from the network whenever it cannot be found locally.
func (s *store) Get(ctx context.Context, mode storage.ModeGet, addr swarm.Address) (ch swarm.Chunk, err error) {
	ch, err = s.Storer.Get(ctx, mode, addr)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// request from network
			data, err := s.retrieval.RetrieveChunk(ctx, addr)
			if err != nil {
				targets := ctx.Value(targetsContextKey)
				if s.recoveryCallback != nil && targets != nil {
					go s.recoveryCallback(ctx, addr)
					return nil, ErrRecoveryAttempt

				}
				return nil, fmt.Errorf("netstore retrieve chunk: %w", err)
			}

			ch = swarm.NewChunk(addr, data)
			if !s.valid(ch) {
				return nil, storage.ErrInvalidChunk
			}

			_, err = s.Storer.Put(ctx, storage.ModePutRequest, ch)
			if err != nil {
				return nil, fmt.Errorf("netstore retrieve put: %w", err)
			}
			return ch, nil
		}
		return nil, fmt.Errorf("netstore get: %w", err)
	}
	return ch, nil
}

// Put stores a given chunk in the local storage.
// returns a storage.ErrInvalidChunk error when
// encountering an invalid chunk.
func (s *store) Put(ctx context.Context, mode storage.ModePut, chs ...swarm.Chunk) (exist []bool, err error) {
	for _, ch := range chs {
		if !s.valid(ch) {
			return nil, storage.ErrInvalidChunk
		}
	}
	return s.Storer.Put(ctx, mode, chs...)
}

// checks if a particular chunk is valid using the built in validators
func (s *store) valid(ch swarm.Chunk) (ok bool) {
	for _, v := range s.validators {
		if ok = v.Validate(ch); ok {
			return true
		}
	}
	return false
}
