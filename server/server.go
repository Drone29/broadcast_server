package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type WSMessage struct {
	MsgType int
	Content []byte
}

type WSServer struct {
	// list of unique clients
	clients      map[*websocket.Conn]bool
	broadcast_q  chan WSMessage
	register_q   chan *websocket.Conn
	unregister_q chan *websocket.Conn
	upgrader     websocket.Upgrader
}

const CLIENTS_BUFFER_SIZE = 10

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
				fmt.Println("WS error sending message to:", c.RemoteAddr())
				// append dead connection to buffered channel
				s.unregister_q <- c
			}
		}(conn)
	}

	wg.Wait() // wait for all goroutines to complete

	return nil
}

func (s *WSServer) run() {
	for {
		select {
		case conn := <-s.register_q:
			s.clients[conn] = true
		case conn := <-s.unregister_q:
			if _, ok := s.clients[conn]; ok {
				conn.Close()
				delete(s.clients, conn)
			}
		case msg := <-s.broadcast_q:
			s.broadcast_message(msg)
		}
	}
}

func NewWSServer() *WSServer {
	return &WSServer{
		clients:      make(map[*websocket.Conn]bool),
		broadcast_q:  make(chan WSMessage, CLIENTS_BUFFER_SIZE),
		register_q:   make(chan *websocket.Conn, CLIENTS_BUFFER_SIZE),
		unregister_q: make(chan *websocket.Conn, CLIENTS_BUFFER_SIZE),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				//todo: check auth
				return true // allow all connections
			},
		},
	}
}

// main handler
func (s *WSServer) handleWSConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WS upgrade error:", err)
		return
	}

	fmt.Println("WS new client connected:", conn.RemoteAddr())
	// enqueue new client
	s.register_q <- conn

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("WS client error:", conn.RemoteAddr(), err)
			// enqueue client to be unregistered
			s.unregister_q <- conn
			break
		}

		fmt.Printf("WS received from %s %s\n", conn.RemoteAddr(), msg)

		// enqueue message for broadcasting
		s.broadcast_q <- WSMessage{MsgType: msgType, Content: msg}
	}
}

// start server
func Start(port int) {
	ws_server := NewWSServer()

	// run server
	go ws_server.run()

	http.HandleFunc("/ws", ws_server.handleWSConnection)
	fmt.Println("WS server started on port", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
