// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mock

import "github.com/ethersphere/bee/pkg/postage"

type optionFunc func(*mockPostage)

// Option is an option passed to a mock postage Service.
type Option interface {
	apply(*mockPostage)
}

func (f optionFunc) apply(r *mockPostage) { f(r) }

// New creates a new mock postage service.
func New(o ...Option) postage.Service {
	m := &mockPostage{}
	for _, v := range o {
		v.apply(m)
	}

	return m
}

// WithMockBatch sets the mock batch on the mock postage service.
func WithMockBatch(id []byte) Option {
	return optionFunc(func(m *mockPostage) {})
}

type mockPostage struct {
	i *postage.StampIssuer
}

func (m *mockPostage) Add(s *postage.StampIssuer) {
	m.i = s
}

func (m *mockPostage) StampIssuers() []*postage.StampIssuer {
	return []*postage.StampIssuer{m.i}
}

func (m *mockPostage) GetStampIssuer(id []byte) (*postage.StampIssuer, error) {
	if m.i != nil {
		return m.i
	}

	// fallback - return
	return postage.NewStampIssuer("test fallback", "test identity", id, 24, 6), nil
}

func (m *mockPostage) Load() error {
	panic("not implemented") // TODO: Implement
}

func (m *mockPostage) Save() error {
	panic("not implemented") // TODO: Implement
}
