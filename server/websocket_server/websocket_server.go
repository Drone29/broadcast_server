package websocket_server

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const CLIENTS_BUFFER_SIZE = 10
const PING_TIMEOUT_S = 10
const PING_INTERVAL_S = 5

type WSMessage struct {
	msg_type int
	data     []byte
}

type WSClientMessage struct {
	client *websocket.Conn
	msg    WSMessage
}

type WSServer struct {
	clients        map[*websocket.Conn]struct{}
	broadcast_q    chan WSMessage
	client_msg_q   chan WSClientMessage
	register_q     chan *websocket.Conn
	unregister_q   chan *websocket.Conn
	quit           chan struct{}
	upgrader       websocket.Upgrader
	active_clients sync.WaitGroup
}

func (s *WSServer) register_new_client(conn *websocket.Conn) {
	s.clients[conn] = struct{}{}
}

func (s *WSServer) unregister_client(conn *websocket.Conn) {
	if _, ok := s.clients[conn]; ok {
		fmt.Println("WS unregistering client", conn.RemoteAddr())
		delete(s.clients, conn)
		conn.Close()
	}
}

func (s *WSServer) broadcast_message(msg WSMessage) error {
	message, err := websocket.NewPreparedMessage(msg.msg_type, msg.data)
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

func (s *WSServer) send_client_msg(client_message WSClientMessage) error {
	conn := client_message.client
	msg := client_message.msg
	if err := conn.WriteMessage(msg.msg_type, msg.data); err != nil {
		fmt.Println("WS write error:", err)
		s.unregister_q <- conn
		return err
	}
	return nil
}

func (s *WSServer) shutdown_gracefully() {
	for conn := range s.clients {
		fmt.Println("WS closing client", conn.RemoteAddr())
		conn.Close()
	}
}

func extend_client_life(conn *websocket.Conn) error {
	return conn.SetReadDeadline(time.Now().Add(PING_TIMEOUT_S * time.Second)) // set timeout
}

func (s *WSServer) run_ping_loop(conn *websocket.Conn, ping_stop chan struct{}) {
	ticker := time.NewTicker(PING_INTERVAL_S * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// send ping
			s.client_msg_q <- WSClientMessage{
				client: conn,
				msg: WSMessage{
					msg_type: websocket.PingMessage,
				},
			}
		case <-ping_stop:
			return
		}
	}
}

func (s *WSServer) configure_client(conn *websocket.Conn) {
	// configure ping
	conn.SetPingHandler(func(appData string) error {
		fmt.Printf("WS received ping %s from %s\n", appData, conn.RemoteAddr())
		// send pong back
		s.client_msg_q <- WSClientMessage{
			client: conn,
			msg: WSMessage{
				msg_type: websocket.PongMessage,
			},
		}
		return extend_client_life(conn) // extend timeout
	})
	// configure pong
	conn.SetPongHandler(func(appData string) error {
		fmt.Printf("WS received pong %s from %s\n", appData, conn.RemoteAddr())
		return extend_client_life(conn) // extend timeout
	})

	extend_client_life(conn) // set timeout for the first time
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
		extend_client_life(conn) // extend timeout

		// enqueue message for broadcasting
		s.broadcast_q <- WSMessage{msg_type: msgType, data: msg}
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
	s.configure_client(conn) // client config (timeout, ping/pong)
	s.active_clients.Add(1)  // increment semaphore
	ping_stop := make(chan struct{})
	go func() {
		go s.run_ping_loop(conn, ping_stop) // ping loop
		s.handle_incoming_messages(conn)    // message handler
		close(ping_stop)                    // signal ping loop to stop
		s.active_clients.Done()             // decrement semaphore
	}()
}

func (s *WSServer) Shutdown() error {
	// signal shutdown
	s.quit <- struct{}{}
	// wait until ws server terminates gracefully
	s.active_clients.Wait()
	return nil
}

func (s *WSServer) Start() {
	for {
		select {
		case conn := <-s.register_q:
			// register new client
			s.register_new_client(conn)
		case conn := <-s.unregister_q:
			// unregister client
			s.unregister_client(conn)
		case msg := <-s.broadcast_q:
			// broadcast message to all clients
			s.broadcast_message(msg)
		case msg := <-s.client_msg_q:
			// send individual message to client
			s.send_client_msg(msg)
		case <-s.quit:
			// graceful shutdown
			s.shutdown_gracefully()
			return
		}
	}
}

func NewWSServer() *WSServer {
	return &WSServer{
		clients:      make(map[*websocket.Conn]struct{}),
		broadcast_q:  make(chan WSMessage, CLIENTS_BUFFER_SIZE),
		client_msg_q: make(chan WSClientMessage, CLIENTS_BUFFER_SIZE),
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
