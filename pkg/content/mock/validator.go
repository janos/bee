// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mock

import (
	"github.com/ethersphere/bee/pkg/swarm"
)

var _ swarm.ChunkValidator = (*Validator)(nil)

// ContentAddressValidator validates that the address of a given chunk
// is the content address of its contents.
type Validator struct {
}

// NewContentAddressValidator constructs a new ContentAddressValidator
func NewValidator() swarm.ChunkValidator {
	return &Validator{}
}

// Validate performs the validation check.
func (v *Validator) Validate(ch swarm.Chunk) (valid bool) {
	return true
}
