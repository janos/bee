// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mock

import (
	"context"
	"errors"
	"sync"

	"github.com/ethersphere/bee/pkg/chunk"
	"github.com/ethersphere/bee/pkg/storage"
	"github.com/ethersphere/bee/pkg/swarm"
	"github.com/ethersphere/bee/pkg/tags"
)

var _ storage.Storer = (*MockStorer)(nil)

type MockStorer struct {
	store            map[string][]byte
	modeSet          map[string]storage.ModeSet
	modeSetMu        sync.Mutex
	pinnedAddress    []swarm.Address // Stores the pinned address
	pinnedCounter    []uint64        // and its respective counter. These are stored as slices to preserve the order.
	pinSetMu         sync.Mutex
	subpull          []storage.Descriptor
	partialInterval  bool
	validator        swarm.ChunkValidator
	tags             *tags.Tags
	morePull         chan struct{}
	mtx              sync.Mutex
	quit             chan struct{}
	baseAddress      []byte
	bins             []uint64
	recoveryCallback chunk.RecoveryHook // this is the callback to be executed when a chunk fails to be retrieved
	deliveryCallback func(swarm.Chunk)  // callback func to be invoked to deliver validated chunks
}

func WithSubscribePullChunks(chs ...storage.Descriptor) Option {
	return optionFunc(func(m *MockStorer) {
		m.subpull = make([]storage.Descriptor, len(chs))
		for i, v := range chs {
			m.subpull[i] = v
		}
	})
}

func WithBaseAddress(a swarm.Address) Option {
	return optionFunc(func(m *MockStorer) {
		m.baseAddress = a.Bytes()
	})
}

func WithTags(t *tags.Tags) Option {
	return optionFunc(func(m *MockStorer) {
		m.tags = t
	})
}

func WithPartialInterval(v bool) Option {
	return optionFunc(func(m *MockStorer) {
		m.partialInterval = v
	})
}

func NewStorer(opts ...Option) *MockStorer {
	s := &MockStorer{
		store:     make(map[string][]byte),
		modeSet:   make(map[string]storage.ModeSet),
		modeSetMu: sync.Mutex{},
		morePull:  make(chan struct{}),
		quit:      make(chan struct{}),
		bins:      make([]uint64, swarm.MaxBins),
	}

	for _, v := range opts {
		v.apply(s)
	}

	return s
}

func NewValidatingStorer(v swarm.Validator, tags *tags.Tags) *MockStorer {
	return &MockStorer{
		store:     make(map[string][]byte),
		modeSet:   make(map[string]storage.ModeSet),
		modeSetMu: sync.Mutex{},
		pinSetMu:  sync.Mutex{},
		validator: v,
		tags:      tags,
	}
}

func NewTagsStorer(tags *tags.Tags) *MockStorer {
	return &MockStorer{
		store:     make(map[string][]byte),
		modeSet:   make(map[string]storage.ModeSet),
		modeSetMu: sync.Mutex{},
		pinSetMu:  sync.Mutex{},
		tags:      tags,
	}
}

func (m *MockStorer) Get(ctx context.Context, mode storage.ModeGet, addr swarm.Address) (ch swarm.Chunk, err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	v, has := m.store[addr.String()]
	if !has {
		return nil, storage.ErrNotFound
	}
	return swarm.NewChunk(addr, v), nil
}

func (m *MockStorer) Put(ctx context.Context, mode storage.ModePut, chs ...swarm.Chunk) (exist []bool, err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, ch := range chs {
		if m.validator != nil {
			if !m.validator.Validate(ch) {
				return nil, storage.ErrInvalidChunk
			}
		}
		m.store[ch.Address().String()] = ch.Data()
		yes, err := m.has(ctx, ch.Address())
		if err != nil {
			exist = append(exist, false)
			continue
		}
		if yes {
			exist = append(exist, true)
		} else {
			po := swarm.Proximity(ch.Address().Bytes(), m.baseAddress)
			m.bins[po]++
			exist = append(exist, false)
		}

	}
	return exist, nil
}

func (m *MockStorer) GetMulti(ctx context.Context, mode storage.ModeGet, addrs ...swarm.Address) (ch []swarm.Chunk, err error) {
	panic("not implemented") // TODO: Implement
}

func (m *MockStorer) has(ctx context.Context, addr swarm.Address) (yes bool, err error) {
	_, has := m.store[addr.String()]
	return has, nil
}

func (m *MockStorer) Has(ctx context.Context, addr swarm.Address) (yes bool, err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.has(ctx, addr)
}

func (m *MockStorer) HasMulti(ctx context.Context, addrs ...swarm.Address) (yes []bool, err error) {
	panic("not implemented") // TODO: Implement
}

func (m *MockStorer) Set(ctx context.Context, mode storage.ModeSet, addrs ...swarm.Address) (err error) {
	m.modeSetMu.Lock()
	m.pinSetMu.Lock()
	defer m.modeSetMu.Unlock()
	defer m.pinSetMu.Unlock()
	for _, addr := range addrs {
		m.modeSet[addr.String()] = mode

		// if mode is set pin, increment the pin counter
		if mode == storage.ModeSetPin {
			var found bool
			for i, ad := range m.pinnedAddress {
				if addr.String() == ad.String() {
					m.pinnedCounter[i] = m.pinnedCounter[i] + 1
					found = true
				}
			}
			if !found {
				m.pinnedAddress = append(m.pinnedAddress, addr)
				m.pinnedCounter = append(m.pinnedCounter, uint64(1))
			}
		}

		// if mode is set unpin, decrement the pin counter and remove the address
		// once it reaches zero
		if mode == storage.ModeSetUnpin {
			for i, ad := range m.pinnedAddress {
				if addr.String() == ad.String() {
					m.pinnedCounter[i] = m.pinnedCounter[i] - 1
					if m.pinnedCounter[i] == 0 {
						copy(m.pinnedAddress[i:], m.pinnedAddress[i+1:])
						m.pinnedAddress[len(m.pinnedAddress)-1] = swarm.NewAddress([]byte{0})
						m.pinnedAddress = m.pinnedAddress[:len(m.pinnedAddress)-1]

						copy(m.pinnedCounter[i:], m.pinnedCounter[i+1:])
						m.pinnedCounter[len(m.pinnedCounter)-1] = uint64(0)
						m.pinnedCounter = m.pinnedCounter[:len(m.pinnedCounter)-1]
					}
				}
			}
		}
	}
	return nil
}

func (m *MockStorer) GetModeSet(addr swarm.Address) (mode storage.ModeSet) {
	m.modeSetMu.Lock()
	defer m.modeSetMu.Unlock()
	if mode, ok := m.modeSet[addr.String()]; ok {
		return mode
	}
	return mode
}

func (m *MockStorer) LastPullSubscriptionBinID(bin uint8) (id uint64, err error) {
	return m.bins[bin], nil
}

func (m *MockStorer) SubscribePull(ctx context.Context, bin uint8, since, until uint64) (<-chan storage.Descriptor, <-chan struct{}, func()) {
	c := make(chan storage.Descriptor)
	done := make(chan struct{})
	stop := func() {
		close(done)
	}
	go func() {
		defer close(c)
		m.mtx.Lock()
		for _, ch := range m.subpull {
			select {
			case c <- ch:
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-m.quit:
				return
			}
		}
		m.mtx.Unlock()

		if m.partialInterval {
			// block since we're at the top of the bin and waiting for new chunks
			select {
			case <-done:
				return
			case <-m.quit:
				return
			case <-ctx.Done():
				return
			case <-m.morePull:

			}
		}

		m.mtx.Lock()
		defer m.mtx.Unlock()

		// iterate on what we have in the iterator
		for _, ch := range m.subpull {
			select {
			case c <- ch:
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-m.quit:
				return
			}
		}

	}()
	return c, m.quit, stop
}

func (m *MockStorer) MorePull(d ...storage.Descriptor) {
	// clear out what we already have in subpull
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.subpull = make([]storage.Descriptor, len(d))
	for i, v := range d {
		m.subpull[i] = v
	}
	close(m.morePull)
}

func (m *MockStorer) SubscribePush(ctx context.Context) (c <-chan swarm.Chunk, stop func()) {
	panic("not implemented") // TODO: Implement
}

func (m *MockStorer) PinnedChunks(ctx context.Context, cursor swarm.Address) (pinnedChunks []*storage.Pinner, err error) {
	m.pinSetMu.Lock()
	defer m.pinSetMu.Unlock()
	if len(m.pinnedAddress) == 0 {
		return pinnedChunks, nil
	}
	for i, addr := range m.pinnedAddress {
		pi := &storage.Pinner{
			Address:    swarm.NewAddress(addr.Bytes()),
			PinCounter: m.pinnedCounter[i],
		}
		pinnedChunks = append(pinnedChunks, pi)
	}
	if pinnedChunks == nil {
		return pinnedChunks, errors.New("pin chunks: leveldb: not found")
	}
	return pinnedChunks, nil
}

func (m *MockStorer) PinInfo(address swarm.Address) (uint64, error) {
	m.pinSetMu.Lock()
	defer m.pinSetMu.Unlock()
	for i, addr := range m.pinnedAddress {
		if addr.String() == address.String() {
			return m.pinnedCounter[i], nil
		}
	}
	return 0, storage.ErrNotFound
}

func (m *MockStorer) WithRecoveryCallBack(rcb chunk.RecoveryHook) {
	m.recoveryCallback = rcb
}

func (m *MockStorer) WithDeliveryCallBack(dcb func(swarm.Chunk)) {
	m.deliveryCallback = dcb
}

func (m *MockStorer) Close() error {
	close(m.quit)
	return nil
}

type Option interface {
	apply(*MockStorer)
}
type optionFunc func(*MockStorer)

func (f optionFunc) apply(r *MockStorer) { f(r) }
