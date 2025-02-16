package server

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

func handleReceivedMsg(ws_conn *websocket.Conn, msgType int, msg []byte) {

}

// main handler
func handleWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // allow all connections
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WS upgrade error:", err)
		return
	}
	defer conn.Close()

	fmt.Println("WS new client connected")

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("WS Read error:", err)
		}

		fmt.Printf("WS received %v %s\n", msgType, msg)

		// handle received message
		handleReceivedMsg(conn, msgType, msg)
	}
}

// start server
func Start(port int) {
	http.HandleFunc("/ws", handleWS)
	fmt.Println("WS server started on port", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
