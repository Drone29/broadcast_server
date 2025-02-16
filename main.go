package main

import (
	"fmt"
	"os"
	"strconv"
)

var port int = 1234

func handleStart() error {
	fmt.Printf("Server listening on port %v\n", port)

	return nil
}

func handleConnect() error {
	fmt.Printf("Connecting to server on port %v\n", port)

	return nil
}

func parseCLI() func() error {
	// panic handler
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("%s\n", r)
			fmt.Printf("Usage: %s <start|connect> [port]\n", os.Args[0])
			fmt.Printf("\tstart - start server instance\n")
			fmt.Printf("\tconnect - connect client to the server\n")
			fmt.Printf("\tport - specify port for the server to listen to, or for a client to connect to [default %v]\n", port)
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
			return handleStart
		case "connect":
			return handleConnect
		default:
			panic(fmt.Sprintf("Unknown command: %s", cmd))
		}
	} else {
		panic("No arguments provided")
	}
}

func main() {
	cmd := parseCLI()
	cmd()
}
