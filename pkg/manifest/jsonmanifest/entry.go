// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonmanifest

import (
	"encoding/json"
	"net/http"

	"github.com/ethersphere/bee/pkg/manifest"
	"github.com/ethersphere/bee/pkg/swarm"
)

// verify jsonEntry implements manifest.Entry.
var _ manifest.Entry = (*jsonEntry)(nil)

// jsonEntry is a JSON representation of a single manifest entry for a jsonManifest.
type jsonEntry struct {
	reference swarm.Address
	name      string
	headers   http.Header
}

// NewEntry creates a new jsonEntry struct and returns it.
func NewEntry(reference swarm.Address, name string, headers http.Header) manifest.Entry {
	return &jsonEntry{
		reference: reference,
		name:      name,
		headers:   headers,
	}
}

// Reference returns the address of the file in the entry.
func (me *jsonEntry) Reference() swarm.Address {
	return me.reference
}

// Name returns the name of the file in the entry.
func (me *jsonEntry) Name() string {
	return me.name
}

// Headers returns the headers for the file in the manifest entry.
func (me *jsonEntry) Headers() http.Header {
	return me.headers
}

// exportEntry is a struct used for marshaling and unmarshaling jsonEntry structs.
type exportEntry struct {
	Reference swarm.Address `json:"reference"`
	Name      string        `json:"name"`
	Headers   http.Header   `json:"headers"`
}

// MarshalJSON implements the json.Marshaler interface.
func (me *jsonEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(exportEntry{
		Reference: me.reference,
		Name:      me.name,
		Headers:   me.headers,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (me *jsonEntry) UnmarshalJSON(b []byte) error {
	e := exportEntry{}
	if err := json.Unmarshal(b, &e); err != nil {
		return err
	}
	me.reference = e.Reference
	me.name = e.Name
	me.headers = e.Headers
	return nil
}
