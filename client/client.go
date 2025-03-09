package client

import (
	"bufio"
	"fmt"
	"os"

	"github.com/gorilla/websocket"
)

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
			fmt.Println("WS reading error", c.conn.RemoteAddr(), err)
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

	active := true
	closeConnection := func() {
		fmt.Println("closing WS connection...")
		active = false
		c.conn.Close()
	}
	// handle received/to-be-sent messages
	for active {
		select {
		case msg := <-c.msg_rx_q:
			//print received message to stdout
			fmt.Printf("Received message: %s\n", string(msg.data))
		case msg := <-c.msg_tx_q:
			// send message from stdin to server
			if err := c.conn.WriteMessage(msg.msg_type, msg.data); err != nil {
				fmt.Println("error sending message:", err)
			}
		case <-c.quit:
			closeConnection()
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

func (c *Client) Disconnect() {
	close(c.quit) //close channel to signal routine to exit
	fmt.Println("Disconnecting WS client...")
}
