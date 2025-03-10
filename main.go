package main

import (
	"broadcast-server/client"
	"broadcast-server/server"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

var (
	port       int
	quit       chan os.Signal
	debug_mode bool
)

// provides common interface for both server and client graceful shutdown
type Graceful interface {
	Shutdown()
}

func init() {
	port = 1234
	quit = make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	debug_mode = os.Getenv("DEBUG_MODE") == "true"
}

func waitForSignalAndShutdown(x Graceful) {
	// wait for terminate and shutdown gracefully
	sgn := <-quit
	fmt.Printf("Signal caught %s, terminating...\n", sgn)
	x.Shutdown()
}

// launch server
func startServer() Graceful {
	fmt.Printf("Server listening on port %v\n", port)
	server := server.Start(server.ServerCfg{
		Port:      port,
		DebugMode: debug_mode,
	})
	return &server
}

// launch client
func connectClient() Graceful {
	fmt.Printf("Connecting to server on port %v\n", port)
	client := client.Connect(fmt.Sprintf("ws://localhost:%d/ws", port))
	return &client
}

func parseCLIAndStart() Graceful {
	// panic handler
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("%s\n", r)
			fmt.Printf("Usage: %s <start|connect> [port]\n", os.Args[0])
			fmt.Printf("\tstart - start server instance\n")
			fmt.Printf("\tconnect - connect client to the server\n")
			fmt.Printf("\tport - specify port for the server to listen to, or for a client to connect to [default %v]\n", port)
			os.Exit(0)
		}
	}()

	if len(os.Args) > 1 {
		cmd := os.Args[1]
		if len(os.Args) > 2 {
			var err error
			port, err = strconv.Atoi(os.Args[2])
			if err != nil {
				panic(fmt.Sprintf("Invalid port: %s %v", os.Args[2], err))
			}
		}
		switch cmd {
		case "start":
			return startServer()
		case "connect":
			return connectClient()
		default:
			panic(fmt.Sprintf("Unknown command: %s", cmd))
		}
	} else {
		panic("No arguments provided")
	}
}

func main() {
	ws_instance := parseCLIAndStart() // server or client instance
	waitForSignalAndShutdown(ws_instance)
}
