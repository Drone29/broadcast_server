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

func sendToAll(msgType int, msg []byte) []*websocket.Conn {
	var deadConns []*websocket.Conn
	clients_mtx.RLock()
	defer clients_mtx.RUnlock()

	for conn := range clients {
		if err := conn.WriteMessage(msgType, msg); err != nil {
			fmt.Println("WS error sending message to:", conn.RemoteAddr())
			// add to list of dead connections
			deadConns = append(deadConns, conn)
		}
	}
	return deadConns
}

func cleanupDeadConnections(deadConns []*websocket.Conn) {
	clients_mtx.Lock()
	defer clients_mtx.Unlock()

	for _, conn := range deadConns {
		conn.Close()
		delete(clients, conn)
	}
}

func broadcastMessage(msgType int, msg []byte) {
	deadConns := sendToAll(msgType, msg)
	cleanupDeadConnections(deadConns)
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
			cleanupDeadConnections([]*websocket.Conn{conn})
			break
		}

		fmt.Printf("WS received from %s %s\n", conn.RemoteAddr(), msg)

		// handle received message
		broadcastMessage(msgType, msg)
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
