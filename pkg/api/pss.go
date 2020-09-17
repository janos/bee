// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/ethersphere/bee/pkg/jsonhttp"
	"github.com/ethersphere/bee/pkg/swarm"
	"github.com/ethersphere/bee/pkg/trojan"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  swarm.ChunkSize,
		WriteBufferSize: swarm.ChunkSize,
	}
	writeDeadline   = 4 * time.Second // write deadline. should be smaller than the shutdown timeout on api close
	targetMaxLength = 2               // max target length in bytes, in order to prevent grieving by excess computation
)

type PssMessage struct {
	Topic   string
	Message string
}

func (s *server) pssPostHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := mux.Vars(r)["topic"]
	topic := trojan.NewTopic(t)

	tg, _ := mux.Vars(r)["targets"]
	var targets trojan.Targets
	tgts := strings.Split(tg, ",")

	for _, v := range tgts {
		target, err := hex.DecodeString(v)
		if err != nil || len(target) > targetMaxLength {
			s.Logger.Debugf("pss send: bad targets: %v", err)
			s.Logger.Error("pss send: bad targets")
			jsonhttp.BadRequest(w, nil)
			return
		}
		targets = append(targets, target)
	}

	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.Logger.Debugf("pss read payload: %v", err)
		s.Logger.Error("pss read payload")
		jsonhttp.InternalServerError(w, nil)
		return
	}

	err = s.Pss.Send(r.Context(), targets, topic, payload)
	if err != nil {
		s.Logger.Debugf("pss send payload: %v. topic: %s", err, t)
		s.Logger.Error("pss send payload")
		jsonhttp.InternalServerError(w, nil)
		return
	}

	jsonhttp.OK(w, nil)
}

func (s *server) pssWsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Logger.Debugf("pss ws: upgrade: %v", err)
		s.Logger.Error("pss ws: cannot upgrade")
		jsonhttp.InternalServerError(w, nil)
		return
	}

	t, _ := mux.Vars(r)["topic"]
	s.wsWg.Add(1)
	go s.pumpWs(conn, t)
}

func (s *server) pumpWs(conn *websocket.Conn, t string) {
	defer s.wsWg.Done()

	var (
		dataC  = make(chan []byte)
		gone   = make(chan struct{})
		topic  = trojan.NewTopic(t)
		ticker = time.NewTicker(s.WsPingPeriod)
		err    error
	)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()
	cleanup := s.Pss.Register(topic, func(_ context.Context, m *trojan.Message) {
		dataC <- m.Payload
	})

	defer cleanup()

	conn.SetCloseHandler(func(code int, text string) error {
		s.Logger.Debugf("pss handler: client gone. code %d message %s", code, text)
		close(gone)
		return nil
	})

	for {
		select {
		case b := <-dataC:
			err = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err != nil {
				s.Logger.Debugf("pss set write deadline: %v", err)
				return
			}

			err = conn.WriteMessage(websocket.BinaryMessage, b)
			if err != nil {
				s.Logger.Debugf("pss write to websocket: %v", err)
				return
			}

		case <-s.quit:
			// shutdown
			err = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err != nil {
				s.Logger.Debugf("pss set write deadline: %v", err)
				return
			}
			err = conn.WriteMessage(websocket.CloseMessage, []byte{})
			if err != nil {
				s.Logger.Debugf("pss write close message: %v", err)
			}
			return
		case <-gone:
			// client gone
			return
		case <-ticker.C:
			err = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err != nil {
				s.Logger.Debugf("pss set write deadline: %v", err)
				return
			}
			if err = conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				// error encountered while pinging client. client probably gone
				return
			}
		}
	}
}
