package peer

import (
	"fmt"
	"time"
)

func (c *Client) Interested() error {
	if err := c.WriteMessage(&Message{ID: MsgInterested}); err != nil {
		return fmt.Errorf("failed to send interested message: %w", err)
	}

	c.Interest = true
	return nil
}

func (c *Client) WaitForUnchoke(timeout time.Duration) error {
	deadline := time.After(timeout)

	for {
		if !c.IsChoked() {
			return nil
		}

		select {
		case <-c.notify:
			continue
		case <-c.closeCh:
			return fmt.Errorf("connection closed while waiting for unchoke: %w", c.ReadErr())
		case <-deadline:
			return fmt.Errorf("timed out waiting for unchoke")
		}
	}
}
