// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ethersphere/bee/pkg/api"
	"github.com/ethersphere/bee/pkg/jsonhttp"
	"github.com/ethersphere/bee/pkg/jsonhttp/jsonhttptest"
	"github.com/ethersphere/bee/pkg/p2p"
	pingpongmock "github.com/ethersphere/bee/pkg/pingpong/mock"
	"github.com/ethersphere/bee/pkg/swarm"
)

func TestPingpong(t *testing.T) {
	rtt := time.Minute
	peerID := "ca1e9f3938cc1425c6061b96ad9eb93e134dfe8734ad490164ef20af9d1cf59c"
	unknownPeerID := "ca1e9f3938cc1425c6061b96ad9eb93e134dfe8734ad490164ef20af9d1cf59e"
	errorPeerID := "77777777"
	testErr := errors.New("test error")

	pingpongService := pingpongmock.New(func(ctx context.Context, address swarm.Address, msgs ...string) (time.Duration, error) {
		if address.String() == errorPeerID {
			return 0, testErr
		}
		if address.String() != peerID {
			return 0, p2p.ErrPeerNotFound
		}
		return rtt, nil
	})

	client, cleanup := newTestServer(t, testServerOptions{
		Pingpong: pingpongService,
	})
	defer cleanup()

	t.Run("ok", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodPost, "/pingpong/"+peerID, nil, http.StatusOK, api.PingpongResponse{
			RTT: rtt,
		})
	})

	t.Run("peer not found", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodPost, "/pingpong/"+unknownPeerID, nil, http.StatusNotFound, jsonhttp.StatusResponse{
			Code:    http.StatusNotFound,
			Message: "peer not found",
		})
	})

	t.Run("error", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodPost, "/pingpong/"+errorPeerID, nil, http.StatusInternalServerError, jsonhttp.StatusResponse{
			Code:    http.StatusInternalServerError,
			Message: http.StatusText(http.StatusInternalServerError), // do not leek internal error
		})
	})

	t.Run("get method not allowed", func(t *testing.T) {
		jsonhttptest.ResponseDirect(t, client, http.MethodGet, "/pingpong/"+peerID, nil, http.StatusMethodNotAllowed, jsonhttp.StatusResponse{
			Code:    http.StatusMethodNotAllowed,
			Message: http.StatusText(http.StatusMethodNotAllowed),
		})
	})
}
