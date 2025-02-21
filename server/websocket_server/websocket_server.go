package websocket_server

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mcuadros/go-defaults" //defaults
)

type wsMessage struct {
	msg_type int
	data     []byte
}

type wsClientMessage struct {
	client *websocket.Conn
	msg    wsMessage
}

type WSServerConfig struct {
	// number of expected clients. Affects the size of message queue
	ExpectedClients int `default:"10"`
	// time after which the client is perceived dead if there was no messages from it
	PingTimeout time.Duration `default:"10s"`
	// time interval for server to sent ping frames to client.SHould be less than PingTimeout
	PingInterval time.Duration `default:"5s"`
	// time allotted for client to respond to Close frame upon graceful shutdown
	CloseGracePeriod time.Duration `default:"10ms"`
	// time allotted for server to deliver Close frame to client
	CloseGraceWriteTimeout time.Duration `default:"10ms"`
}

type WSServer struct {
	clients        map[*websocket.Conn]struct{}
	broadcast_q    chan wsMessage
	client_msg_q   chan wsClientMessage
	register_q     chan *websocket.Conn
	unregister_q   chan *websocket.Conn
	quit           chan struct{}
	upgrader       websocket.Upgrader
	active_clients sync.WaitGroup
	config         *WSServerConfig
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

func (s *WSServer) broadcast_message(msg wsMessage) error {
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

func (s *WSServer) send_client_msg(client_message wsClientMessage) error {
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
		// send Close control msg to client
		close_msg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "Bye")
		if err := conn.WriteControl(websocket.CloseMessage, close_msg, time.Now().Add(s.config.CloseGraceWriteTimeout)); err != nil {
			fmt.Printf("WS client %s close write error %v\n", conn.RemoteAddr(), err)
		}
		// give handle_incoming_messages some time to break from loop
		time.Sleep(s.config.CloseGracePeriod)
		conn.Close()
	}
}

func (s *WSServer) extend_client_life(conn *websocket.Conn) error {
	return conn.SetReadDeadline(time.Now().Add(s.config.PingTimeout)) // set timeout
}

func (s *WSServer) run_ping_loop(conn *websocket.Conn, ping_stop chan struct{}) {
	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// send ping
			s.client_msg_q <- wsClientMessage{
				client: conn,
				msg: wsMessage{
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
		s.client_msg_q <- wsClientMessage{
			client: conn,
			msg: wsMessage{
				msg_type: websocket.PongMessage,
			},
		}
		return s.extend_client_life(conn) // extend timeout
	})
	// configure pong
	conn.SetPongHandler(func(appData string) error {
		fmt.Printf("WS received pong %s from %s\n", appData, conn.RemoteAddr())
		return s.extend_client_life(conn) // extend timeout
	})

	s.extend_client_life(conn) // set timeout for the first time
}

func (s *WSServer) handle_incoming_messages(conn *websocket.Conn) {
	// enqueue new client
	s.register_q <- conn

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				fmt.Println("WS read error", conn.RemoteAddr(), err)
			}
			break
		}

		fmt.Printf("WS from %s received %s\n", conn.RemoteAddr(), msg)
		s.extend_client_life(conn) // extend timeout

		// enqueue message for broadcasting
		s.broadcast_q <- wsMessage{msg_type: msgType, data: msg}
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

// create new WSServer instance
func NewWSServer(config *WSServerConfig) *WSServer {
	if config.PingTimeout < config.PingInterval {
		panic("ping timeout < ping interval")
	}
	return &WSServer{
		clients:      make(map[*websocket.Conn]struct{}),
		broadcast_q:  make(chan wsMessage, config.ExpectedClients),
		client_msg_q: make(chan wsClientMessage, config.ExpectedClients),
		register_q:   make(chan *websocket.Conn, config.ExpectedClients),
		unregister_q: make(chan *websocket.Conn, config.ExpectedClients),
		quit:         make(chan struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				//todo: check auth
				return true // allow all connections
			},
		},
		config: config,
	}
}

// create new WSServerConfig with defaults
func NewWSServerConfig() *WSServerConfig {
	cfg := &WSServerConfig{}
	defaults.SetDefaults(cfg)
	return cfg
}
