package peer

import "fmt"

func (c *Client) Interested() error {
	if err := c.WriteMessage(&Message{ID: MsgInterested}); err != nil {
		return fmt.Errorf("failed to send interested message: %w", err)
	}

	c.Interest = true
	return nil
}

func (c *Client) WaitForUnchoke() error {
	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return fmt.Errorf("failed during unchoke wait: %w", err)
		}

		if msg.KeepAlive {
			continue
		}

		switch msg.ID {
		case MsgUnchoke:
			c.Choked = false
			return nil
		case MsgChoke:
			c.Choked = true
		case MsgBitfield:
			bf, err := GetBitfield(msg)
			if err != nil {
				return fmt.Errorf("failed to read bitfield: %w", err)
			}

			c.Bitfield = bf
		}
	}
}
