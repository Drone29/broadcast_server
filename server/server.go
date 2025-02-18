package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	// list of unique clients
	clients     map[*WSClient]bool
	clients_mtx = sync.RWMutex{}
	ws_upgrader = websocket.Upgrader{}
)

// initialize variables
func init() {
	clients = make(map[*WSClient]bool)
	ws_upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			//todo: check auth
			return true // allow all connections
		},
	}
}

func addClientToList(conn *WSClient) {
	clients_mtx.Lock()
	defer clients_mtx.Unlock()
	clients[conn] = true
}

func closeSingleConnection(conn *WSClient) {
	deadConns := make(chan *WSClient, 1)
	deadConns <- conn
	close(deadConns)
	cleanupDeadConnections(deadConns)
}

func cleanupDeadConnections(deadConns <-chan *WSClient) {
	clients_mtx.Lock()
	defer clients_mtx.Unlock()

	for conn := range deadConns {
		fmt.Println("WS closing", conn.RemoteAddr())
		conn.Close()
		delete(clients, conn)
	}
}

func getClients() []*WSClient {
	clients_mtx.RLock()
	defer clients_mtx.RUnlock()
	// copy to a slice to ensure we're operating on a stable list of connections
	connections := make([]*WSClient, 0, len(clients))
	for conn := range clients {
		connections = append(connections, conn)
	}
	return connections
}

func broadcastMessage(msgType int, msg []byte) <-chan *WSClient {
	var wg sync.WaitGroup
	connections := getClients()
	// create a buffered channel for dead connections
	deadConns := make(chan *WSClient, len(connections))

	message, err := PrepareMessage(msgType, msg)
	if err != nil {
		fmt.Println("WS error preparing message", err)
		return deadConns
	}

	for _, conn := range connections {
		wg.Add(1) // increment semaphore
		// execute in a goroutine
		go func(c *WSClient) {
			defer wg.Done() // decrement semaphore
			if err := c.WritePreparedMessage(message); err != nil {
				fmt.Println("WS error sending message to:", c.RemoteAddr())
				// append dead connection to buffered channel
				deadConns <- c
			}
		}(conn)
	}

	// wait until all messages are sent to all clients and close the channel
	go func() {
		wg.Wait()
		close(deadConns)
	}()

	return deadConns
}

// main handler
func handleWSConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := ws_upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WS upgrade error:", err)
		return
	}

	client := NewWSClient(conn)

	fmt.Println("WS new client connected:", conn.RemoteAddr())
	// store client in the set
	addClientToList(client)

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("WS client error:", conn.RemoteAddr(), err)
			closeSingleConnection(client)
			break
		}

		fmt.Printf("WS received from %s %s\n", conn.RemoteAddr(), msg)

		// handle received message
		deadConns := broadcastMessage(msgType, msg)
		cleanupDeadConnections(deadConns)
	}
}

// start server
func Start(port int) {
	http.HandleFunc("/ws", handleWSConnection)
	fmt.Println("WS server started on port", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
