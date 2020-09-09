package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ethersphere/bee/pkg/trojan"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
//type Client struct {
//hub *Hub

//// The websocket connection.
//conn *websocket.Conn

//// Buffered channel of outbound messages.
//send chan []byte
//}

// serveWs handles websocket requests from the peer.
//func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
//conn, err := upgrader.Upgrade(w, r, nil)
//if err != nil {
//log.Println(err)
//return
//}
//client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
//client.hub.register <- client

//// Allow collection of memory referenced by the caller by doing all work in
//// new goroutines.
//go client.writePump()
//go client.readPump()
//}

func (s *server) pssPostHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("pss upgrading request")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("error", err)
		return
	}

	t, ok := mux.Vars(r)["topic"]
	if !ok {
		panic(err)
	}

	handler := func(ctx context.Context, m *trojan.Message) error {
		fmt.Println("pss writing to handler")
		fmt.Println("writing payload", string(m.Payload))
		conn.WriteMessage(websocket.TextMessage, m.Payload)
		return nil
	}
	topic := trojan.NewTopic(t)
	s.Pss.Register(topic, handler)
	fmt.Println(topic)
	fmt.Println(conn)
	//client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	//client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.

}
