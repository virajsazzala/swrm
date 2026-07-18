package peer

import (
	"encoding/binary"
	"fmt"
)

func (c *Client) Request(p int, b int, l int) error {
	/*
		piece index  - 4 bytes
		begin offset - 4 bytes
		block length - 4 bytes
	*/

	msg := make([]byte, 12)
	binary.BigEndian.PutUint32(msg[:4], uint32(p))
	binary.BigEndian.PutUint32(msg[4:8], uint32(b))
	binary.BigEndian.PutUint32(msg[8:], uint32(l))

	err := c.WriteMessage(&Message{ID: MsgRequest, Payload: msg})
	if err != nil {
		return fmt.Errorf("failed to send request to client: %w", err)
	}

	return nil
}
