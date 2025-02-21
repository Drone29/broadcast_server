package server

import (
	"broadcast-server/server/websocket_server"
	"context"
	"fmt"
	"net/http"
	"time"
)

type WSServer = websocket_server.WSServer

type Server struct {
	ws_server   *WSServer
	http_server *http.Server
}

// start server
func Start(port int) Server {
	ws_cfg := websocket_server.NewWSServerConfig()
	ws_server := websocket_server.NewWSServer(ws_cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws_server.HandleConnection)
	http_server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	fmt.Println("WS server started on port", port)
	// run ws server
	go ws_server.Start()
	// run http server in separate thread
	go func() {
		if err := http_server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	return Server{ws_server: ws_server, http_server: http_server}
}

func (s *Server) Shutdown() {
	fmt.Println("Shutting down WS...")
	s.ws_server.Shutdown()
	fmt.Println("Shutting down HTTP...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.http_server.Shutdown(ctx); err != nil {
		fmt.Println("HTTP shutdown error:", err)
	}
}
