# Broadcast Server

## Description

Implemented as a WebSocket server, the server allows clients to connect to it, sends messages that will be broadcasted to all connected clients

The application can also act as a WS client and can be used to connect to that server

## Usage

Start server

```sh
broadcast-server start
```

Connect client to the server

```sh
broadcast-server connect
```

The port can be specified as a 2nd argument:

```sh
broadcast-server start 1234
```

```sh
broadcast-server connect 1234
```

### Server mode

In server mode, the application listens to incoming connections from the clients and broadcasts received messages to all connected clients.  
The server keeps track of connected clients by maintaining a list of them and monitoring their status with ping-pong frames sent at the predefined intervals.  
Should any client disappear or explicitly close the connection, the server will remove it from the list and will no longer attempt to communicate with it.  
> **NOTE:**  
For security reasons, WS connections to the server are not allowed from `localhost`, and from any hosts different from the one where the server is hosted, by default (see [CheckOrigin](https://github.com/gorilla/websocket/blob/5e002381133d322c5f1305d171f3bdd07decf229/server.go#L67))  
To override this, use env variable `DEBUG_MODE` and launch the server as follows:

```sh
DEBUG_MODE=true broadcast-server start
```

### Client mode

In client mode, upon connecting to the server, user can type their messages in the terminal and send them to the server by hitting `Enter`.  
The upcoming messages are displayed in the same terminal.

# Roadmap link

https://roadmap.sh/projects/broadcast-server