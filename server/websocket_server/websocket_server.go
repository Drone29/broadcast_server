package websocket_server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

const CLIENTS_BUFFER_SIZE = 10

type WSMessage struct {
	MsgType int
	Content []byte
}

type WSServer struct {
	clients      map[*websocket.Conn]struct{}
	broadcast_q  chan WSMessage
	register_q   chan *websocket.Conn
	unregister_q chan *websocket.Conn
	quit         chan struct{}
	upgrader     websocket.Upgrader
}

func (s *WSServer) unregister_client(conn *websocket.Conn) {
	if _, ok := s.clients[conn]; ok {
		fmt.Println("WS unregistering client", conn.RemoteAddr())
		delete(s.clients, conn)
		conn.Close()
	}
}

func (s *WSServer) broadcast_message(msg WSMessage) error {
	message, err := websocket.NewPreparedMessage(msg.MsgType, msg.Content)
	if err != nil {
		fmt.Println("WS error preparing message", err)
		return err
	}
	var wg sync.WaitGroup

	for conn := range s.clients {
		wg.Add(1) // increment semaphore
		// execute in a goroutine
		go func(c *websocket.Conn) {
			defer wg.Done() // decrement semaphore
			if err := c.WritePreparedMessage(message); err != nil {
				fmt.Println("WS write error:", err)
				// append dead connection to buffered channel
				s.unregister_q <- c
			}
		}(conn)
	}

	wg.Wait() // wait for all goroutines to complete

	return nil
}

func (s *WSServer) shutdown_gracefully() {
	for conn := range s.clients {
		fmt.Println("WS closing client", conn.RemoteAddr())
		conn.Close()
	}
	close(s.quit) //close channel to indicate we're done
}

func (s *WSServer) Start() {
	for {
		select {
		case conn := <-s.register_q:
			s.clients[conn] = struct{}{}
		case conn := <-s.unregister_q:
			s.unregister_client(conn)
		case msg := <-s.broadcast_q:
			s.broadcast_message(msg)
		case <-s.quit:
			s.shutdown_gracefully()
			return
		}
	}
}

func (s *WSServer) handle_incoming_messages(conn *websocket.Conn) {
	// enqueue new client
	s.register_q <- conn

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil && err != websocket.ErrCloseSent {
			fmt.Println("WS read error:", err)
			break
		}

		fmt.Printf("WS from %s received %s\n", conn.RemoteAddr(), msg)

		// enqueue message for broadcasting
		s.broadcast_q <- WSMessage{MsgType: msgType, Content: msg}
	}
	// enqueue client to be unregistered
	s.unregister_q <- conn
}

// main handler
func (s *WSServer) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WS upgrade error:", err)
		return
	}

	fmt.Println("WS new client connected:", conn.RemoteAddr())
	s.handle_incoming_messages(conn)
}

func (s *WSServer) Shutdown() error {
	// signal shutdown
	s.quit <- struct{}{}
	// wait until ws server terminates gracefully
	<-s.quit
	return nil
}

func NewWSServer() *WSServer {
	return &WSServer{
		clients:      make(map[*websocket.Conn]struct{}),
		broadcast_q:  make(chan WSMessage, CLIENTS_BUFFER_SIZE),
		register_q:   make(chan *websocket.Conn, CLIENTS_BUFFER_SIZE),
		unregister_q: make(chan *websocket.Conn, CLIENTS_BUFFER_SIZE),
		quit:         make(chan struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				//todo: check auth
				return true // allow all connections
			},
		},
	}
}
