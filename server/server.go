package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	// list of unique clients
	clients     map[*websocket.Conn]bool
	clients_mtx = sync.RWMutex{}
	ws_upgrader = websocket.Upgrader{}
)

// initialize variables
func init() {
	clients = make(map[*websocket.Conn]bool)
	ws_upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			//todo: check auth
			return true // allow all connections
		},
	}
}

func addClientToList(conn *websocket.Conn) {
	clients_mtx.Lock()
	defer clients_mtx.Unlock()
	clients[conn] = true
}

func closeSingleConnection(conn *websocket.Conn) {
	deadConns := make(chan *websocket.Conn, 1)
	deadConns <- conn
	close(deadConns)
	cleanupDeadConnections(deadConns)
}

func cleanupDeadConnections(deadConns <-chan *websocket.Conn) {
	clients_mtx.Lock()
	defer clients_mtx.Unlock()

	for conn := range deadConns {
		fmt.Println("WS closing", conn.RemoteAddr())
		conn.Close()
		delete(clients, conn)
	}
}

func getClients() []*websocket.Conn {
	clients_mtx.RLock()
	defer clients_mtx.RUnlock()
	// copy to a slice to ensure we're operating on a stable list of connections
	connections := make([]*websocket.Conn, 0, len(clients))
	for conn := range clients {
		connections = append(connections, conn)
	}
	return connections
}

func broadcastMessage(msgType int, msg []byte) <-chan *websocket.Conn {
	var wg sync.WaitGroup
	connections := getClients()
	// create a buffered channel for dead connections
	deadConns := make(chan *websocket.Conn, len(connections))

	for _, conn := range connections {
		wg.Add(1) // increment semaphore
		// execute in a goroutine
		go func(c *websocket.Conn) {
			defer wg.Done() // decrement semaphore
			if err := c.WriteMessage(msgType, msg); err != nil {
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

	fmt.Println("WS new client connected:", conn.RemoteAddr())
	// store client in the set
	addClientToList(conn)

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("WS client error:", conn.RemoteAddr(), err)
			closeSingleConnection(conn)
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
