package peer

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	MsgChoke         = 0
	MsgUnchoke       = 1
	MsgInterested    = 2
	MsgNotInterested = 3
	MsgHave          = 4
	MsgBitfield      = 5
	MsgRequest       = 6
	MsgPiece         = 7
	MsgCancel        = 8
)

const maxMessageLength = 1 << 20 // 1MB

type Message struct {
	ID        byte
	Payload   []byte
	KeepAlive bool
}

func (c *Client) ReadMessage() (*Message, error) {
	/*
		message format:

		4byte length  mid   payload
		-------------+----+------------
		 00 00 00 00 | 00 | 00 00 ....
		-------------+----+------------

	*/
	var length uint32
	err := binary.Read(c.Conn, binary.BigEndian, &length)
	if err != nil {
		return nil, fmt.Errorf("failed to read length header: %w", err)
	}

	if length == 0 {
		return &Message{Payload: nil, KeepAlive: true}, nil
	}

	if length > maxMessageLength {
		return nil, fmt.Errorf("message length %d exceeds max allowed %d", length, maxMessageLength)
	}

	data := make([]byte, length)
	_, err = io.ReadFull(c.Conn, data)
	if err != nil {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}

	id := data[0]

	return &Message{ID: id, Payload: data[1:], KeepAlive: false}, nil
}

func (c *Client) WriteMessage(msg *Message) error {
	var data []byte
	if !msg.KeepAlive {
		length := 4 + 1 + len(msg.Payload)
		data = make([]byte, length)

		binary.BigEndian.PutUint32(data[0:4], uint32(len(msg.Payload)+1))
		data[4] = msg.ID
		copy(data[5:], msg.Payload)
	} else {
		data = make([]byte, 4)
	}

	_, err := c.Conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write message to client: %w", err)
	}

	return nil
}
