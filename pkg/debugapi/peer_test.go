// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debugapi_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ethersphere/bee/pkg/debugapi"
	"github.com/ethersphere/bee/pkg/jsonhttp"
	"github.com/ethersphere/bee/pkg/jsonhttp/jsonhttptest"
	"github.com/ethersphere/bee/pkg/p2p"
	"github.com/ethersphere/bee/pkg/p2p/mock"
	"github.com/ethersphere/bee/pkg/swarm"
	ma "github.com/multiformats/go-multiaddr"
)

func TestConnect(t *testing.T) {
	underlay := "/ip4/127.0.0.1/tcp/7070/p2p/16Uiu2HAkx8ULY8cTXhdVAcMmLcH9AsTKz6uBQ7DPLKRjMLgBVYkS"
	errorUnderlay := "/ip4/127.0.0.1/tcp/7070/p2p/16Uiu2HAkw88cjH2orYrB6fDui4eUNdmgkwnDM8W681UbfsPgM9QY"
	overlay, _ := swarm.ParseHexAddress("ca1e9f3938cc1425c6061b96ad9eb93e134dfe8734ad490164ef20af9d1cf59c")
	testErr := errors.New("test error")

	client, cleanup := newTestServer(t, testServerOptions{
		P2P: mock.New(mock.WithConnectFunc(func(ctx context.Context, addr ma.Multiaddr) (swarm.Address, error) {
			if addr.String() == errorUnderlay {
				return swarm.Address{}, testErr
			}
			return overlay, nil
		})),
	})
	defer cleanup()

	t.Run("ok", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodPost, "/connect"+underlay, nil, http.StatusOK, debugapi.PeerConnectResponse{
			Address: overlay.String(),
		})
	})

	t.Run("error", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodPost, "/connect"+errorUnderlay, nil, http.StatusInternalServerError, jsonhttp.StatusResponse{
			Code:    http.StatusInternalServerError,
			Message: testErr.Error(),
		})
	})

	t.Run("get method not allowed", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodGet, "/connect"+underlay, nil, http.StatusMethodNotAllowed, jsonhttp.StatusResponse{
			Code:    http.StatusMethodNotAllowed,
			Message: http.StatusText(http.StatusMethodNotAllowed),
		})
	})
}

func TestDisconnect(t *testing.T) {
	address := "ca1e9f3938cc1425c6061b96ad9eb93e134dfe8734ad490164ef20af9d1cf59c"
	unknwonAdddress := "ca1e9f3938cc1425c6061b96ad9eb93e134dfe8734ad490164ef20af9d1cf59e"
	errorAddress := "33333333"
	testErr := errors.New("test error")

	client, cleanup := newTestServer(t, testServerOptions{
		P2P: mock.New(mock.WithDisconnectFunc(func(addr swarm.Address) error {
			switch addr.String() {
			case address:
				return nil
			case errorAddress:
				return testErr
			default:
				return p2p.ErrPeerNotFound
			}
		})),
	})
	defer cleanup()

	t.Run("ok", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodDelete, "/peers/"+address, nil, http.StatusOK, jsonhttp.StatusResponse{
			Code:    http.StatusOK,
			Message: http.StatusText(http.StatusOK),
		})
	})

	t.Run("unknown", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodDelete, "/peers/"+unknwonAdddress, nil, http.StatusBadRequest, jsonhttp.StatusResponse{
			Code:    http.StatusBadRequest,
			Message: "peer not found",
		})
	})

	t.Run("error", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodDelete, "/peers/"+errorAddress, nil, http.StatusInternalServerError, jsonhttp.StatusResponse{
			Code:    http.StatusInternalServerError,
			Message: testErr.Error(),
		})
	})
}
