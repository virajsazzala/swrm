package peer

import (
	"fmt"
	"time"
)

func (c *Client) readLoop() {
	for {
		c.Conn.SetReadDeadline(time.Now().Add(readIdleTimeout))

		msg, err := c.ReadMessage()
		if err != nil {
			c.setReadErr(fmt.Errorf("read failed: %w", err))
			c.Close()
			return
		}

		if msg.KeepAlive {
			continue
		}

		switch msg.ID {
		case MsgChoke:
			c.setChoked(true)

		case MsgUnchoke:
			c.setChoked(false)

		case MsgBitfield:
			bf, err := GetBitfield(msg)
			if err != nil {
				c.setReadErr(fmt.Errorf("failed to read bitfield: %w", err))
				c.Close()
				return
			}
			c.setBitfield(bf)

		case MsgHave:
			pieceIndex, err := parseHave(msg)
			if err != nil {
				c.setReadErr(fmt.Errorf("failed to read have: %w", err))
				c.Close()
				return
			}
			c.setPiece(pieceIndex)

		case MsgPiece:
			block, err := parseBlock(msg)
			if err != nil {
				c.setReadErr(fmt.Errorf("failed to parse block: %w", err))
				c.Close()
				return
			}

			select {
			case c.blocks <- block:
			case <-c.closeCh:
				return
			}
		}
	}
}
