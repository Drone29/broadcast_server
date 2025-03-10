package client

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const graceful_timeout = time.Millisecond * 10

type wsMessage struct {
	msg_type int
	data     []byte
}

type Client struct {
	conn     *websocket.Conn
	msg_rx_q chan wsMessage
	msg_tx_q chan wsMessage
	quit     chan struct{}
}

// read messages from server
func (c *Client) receiveMessages() {
	for {
		// read message
		msg_type, msg, err := c.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				fmt.Println("WS reading error", c.conn.RemoteAddr(), err)
			}
			close(c.msg_rx_q) // close channel and exit
			return
		} else {
			// send to queue
			c.msg_rx_q <- wsMessage{
				msg_type: msg_type,
				data:     msg,
			}
		}
	}
}

// send messages to server
func (c *Client) sendMessages() {
	fmt.Println("Type your messages here and hit Enter to send them")
	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("error reading input:", err)
			close(c.msg_tx_q) //close channel and exit
			return
		} else {
			// cut \n from input
			input = input[:len(input)-1]
			// send to queue
			c.msg_tx_q <- wsMessage{
				msg_type: websocket.TextMessage,
				data:     []byte(input),
			}
		}
	}
}

func (c *Client) handleMessages() {
	// launch sender and receiver
	go c.receiveMessages()
	go c.sendMessages()

	// handle received/to-be-sent messages
	for {
		select {
		case msg, ok := <-c.msg_rx_q:
			if ok {
				//print received message to stdout
				fmt.Printf("Received message: %s\n", string(msg.data))
			}
		case msg, ok := <-c.msg_tx_q:
			if ok {
				// send message from stdin to server
				if err := c.conn.WriteMessage(msg.msg_type, msg.data); err != nil {
					fmt.Println("error sending message:", err)
				}
			}
		case <-c.quit:
			close_msg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "Bye")
			if err := c.conn.WriteControl(websocket.CloseMessage, close_msg, time.Now().Add(graceful_timeout)); err != nil {
				fmt.Printf("WS client %s close write error %v\n", c.conn.RemoteAddr(), err)
			} else {
				//graceful exit, wait for server to respond
				time.Sleep(graceful_timeout)
			}
			close(c.quit)
			return
		}
	}
}

func Connect(url string) Client {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		panic(fmt.Sprintf("error dialing %s: %v\n", url, err))
	}
	client := Client{
		conn:     conn,
		msg_rx_q: make(chan wsMessage, 1),
		msg_tx_q: make(chan wsMessage, 1),
		quit:     make(chan struct{}),
	}
	go client.handleMessages()
	return client
}

func (c *Client) Shutdown() {
	c.quit <- struct{}{} //signal shutdown
	fmt.Println("Disconnecting WS client...")
	<-c.quit
	c.conn.Close()
}
