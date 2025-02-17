package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	// list of unique clients
	clients     map[*websocket.Conn]struct{}
	clients_mtx = sync.Mutex{}
	ws_upgrader = websocket.Upgrader{}
)

// initialize variables
func init() {
	clients = make(map[*websocket.Conn]struct{})
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
	clients[conn] = struct{}{}
}

func disconnectAndRemoveClientFromList(conn *websocket.Conn) {
	clients_mtx.Lock()
	defer clients_mtx.Unlock()
	conn.Close()
	delete(clients, conn)
}

func broadcastMessage(msgType int, msg []byte) {
	clients_mtx.Lock()
	defer clients_mtx.Unlock()
	for conn := range clients {
		err := conn.WriteMessage(msgType, msg)
		if err != nil {
			fmt.Println("WS error sending message to:", conn.RemoteAddr())
			// remove dead connection
			disconnectAndRemoveClientFromList(conn)
		}
	}
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
			fmt.Println("WS read error:", conn.RemoteAddr(), err)
			disconnectAndRemoveClientFromList(conn)
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
