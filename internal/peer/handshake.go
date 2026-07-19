package peer

import (
	"bytes"
	"fmt"
	"io"
	"time"
)

func (c *Client) Handshake(infoHash [20]byte, peerID [20]byte) error {
	/*
	   Byte:  0 |        1..19        | 20..27 |    28..47    |    48..67
	         +---+---------------------+--------+--------------+-------------+
	         |19 | BitTorrent protocol |   0s   |  info_hash   |   peer_id   |
	         +---+---------------------+--------+--------------+-------------+
	*/

	const protocol = "BitTorrent protocol"

	/* build message */
	var msg [68]byte
	msg[0] = 19               // pstrlen
	copy(msg[1:20], protocol) // protocol string
	// bytes 20-27 remain 0 (reserved bytes)
	copy(msg[28:48], infoHash[:]) // info_hash
	copy(msg[48:68], peerID[:])   // peer_id

	_ = c.Conn.SetDeadline(time.Now().Add(5 * time.Second))

	// write msg to conn
	_, err := c.Conn.Write(msg[:])
	if err != nil {
		return fmt.Errorf("failed to write handshake response: %w", err)
	}

	// read msg from conn
	var resp [68]byte
	_, err = io.ReadFull(c.Conn, resp[:])
	if err != nil {
		return fmt.Errorf("failed to read handshake response: %w", err)
	}

	// validate msg
	if int(resp[0]) != 19 {
		return fmt.Errorf(`expected protocol string length 19, received %v`, int(resp[0]))
	}
	if string(resp[1:20]) != protocol {
		return fmt.Errorf(`expected protocol string 'BitTorrent protocol', received %s`, string(resp[1:20]))
	}

	if !bytes.Equal(infoHash[:], resp[28:48]) {
		return fmt.Errorf("incorrect info_hash received")
	}

	// store peerid
	var id [20]byte
	copy(id[:], resp[48:68])
	c.PeerID = id

	_ = c.Conn.SetDeadline(time.Time{})
	return nil
}
