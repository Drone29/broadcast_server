# Broadcast Server

## Description

Allows clients to connect to it, sends messages that will be broadcasted to all connected clients

## Usage

Start server

```sh
broadcast-server start
```

Connect client to the server

```sh
broadcast-server connect
```

A port can be specified as a 2nd argument:

```sh
broadcast-server start 1234
```

```sh
broadcast-server connect 1234
```

> **NOTE:**  
For security reasons, WS connections to the server are not allowed from `localhost`, and from any hosts different from the one where the server is hosted, by default (see [CheckOrigin](https://github.com/gorilla/websocket/blob/5e002381133d322c5f1305d171f3bdd07decf229/server.go#L67))  
To override this, use env variable `DEBUG_MODE` and launch the server as follows:

```sh
DEBUG_MODE=true broadcast-server start
```

# Roadmap link

https://roadmap.sh/projects/broadcast-server