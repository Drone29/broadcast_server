package server

import (
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	conn *websocket.Conn
	mtx  sync.Mutex
}

func NewWSClient(conn *websocket.Conn) *WSClient {
	return &WSClient{
		conn: conn,
		mtx:  sync.Mutex{},
	}
}

func PrepareMessage(msgType int, msg []byte) (*websocket.PreparedMessage, error) {
	return websocket.NewPreparedMessage(msgType, msg)
}

// func (c *WSClient) WriteMessage(messageType int, data []byte) error {
// 	c.mtx.Lock()
// 	defer c.mtx.Unlock()
// 	return c.conn.WriteMessage(messageType, data)
// }

func (c *WSClient) WritePreparedMessage(pm *websocket.PreparedMessage) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.conn.WritePreparedMessage(pm)
}

func (c *WSClient) ReadMessage() (messageType int, p []byte, err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.conn.ReadMessage()
}

func (c *WSClient) Close() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.conn.Close()
}

func (c *WSClient) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
