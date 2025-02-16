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

# Roadmap link

https://roadmap.sh/projects/broadcast-server