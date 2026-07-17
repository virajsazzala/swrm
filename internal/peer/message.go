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
	var mlen uint32
	err := binary.Read(c.Conn, binary.BigEndian, &mlen)
	if err != nil {
		return nil, fmt.Errorf("failed to read length header: %w", err)
	}

	if mlen == 0 {
		return &Message{Payload: nil, KeepAlive: true}, nil
	}
	msg := make([]byte, mlen)
	_, err = io.ReadFull(c.Conn, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}

	mid := msg[0]

	return &Message{ID: mid, Payload: msg[1:], KeepAlive: false}, nil
}

func (c *Client) WriteMessage(msg *Message) error {
	var wmsg []byte
	if !msg.KeepAlive {
		wlen := 4 + 1 + len(msg.Payload)
		wmsg = make([]byte, wlen)

		binary.BigEndian.PutUint32(wmsg[0:4], uint32(len(msg.Payload)+1))
		wmsg[4] = msg.ID
		copy(wmsg[5:], msg.Payload)
	} else {
		wmsg = make([]byte, 4)
	}

	_, err := c.Conn.Write(wmsg)
	if err != nil {
		return fmt.Errorf("failed to write message to client: %w", err)
	}

	return nil
}
