package peer

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"

	"github.com/virajsazzala/swrm/internal/torrent"
)

type Block struct {
	Index int
	Begin int
	Data  []byte
}

type Piece struct {
	ID   int
	Data []byte
}

func parseBlock(m *Message) (*Block, error) {
	if m.ID != MsgPiece {
		return nil, fmt.Errorf("message is not a piece")
	}

	if len(m.Payload) < 8 {
		return nil, fmt.Errorf("piece payload is too short")
	}

	return &Block{
		Index: int(binary.BigEndian.Uint32(m.Payload[0:4])),
		Begin: int(binary.BigEndian.Uint32(m.Payload[4:8])),
		Data:  m.Payload[8:],
	}, nil
}

func (c *Client) downloadPiece(pid int, pln int) ([]byte, error) {
	buf := make([]byte, pln)
	const bs = 16 * 1024

	// for i:=1; i<=bs; i++ {
	for i := 0; i < pln; i += bs {
		bl := min(bs, pln-i)

		err := c.Request(pid, i, bl)
		if err != nil {
			return nil, fmt.Errorf("failed to get block %v for piece %v", i, pid)
		}

	waitlp:
		for {
			msg, err := c.ReadMessage()
			if err != nil {
				return nil, fmt.Errorf("failed to read piece %v, block %v: %v\n", pid, i, err)
			}

			if msg.KeepAlive {
				continue
			}

			switch msg.ID {
			case MsgPiece:
				pb, err := parseBlock(msg)
				if err != nil {
					return nil, fmt.Errorf("failed to parse block for piece %d: %v", pid, err)
				}

				if pb.Index != pid {
					return nil, fmt.Errorf("invalid piece block, request %v, got %v", pid, pb.Index)
				}

				if pb.Begin != i {
					return nil, fmt.Errorf("invalid piece block offset, request %v, got %v", i, pb.Begin)
				}

				if pb.Begin+len(pb.Data) > len(buf) {
					return nil, fmt.Errorf("piece block exceeds buffer")
				}

				if len(pb.Data) != bl {
					return nil, fmt.Errorf("expected %d bytes, got %d", bl, len(pb.Data))
				}

				copy(buf[pb.Begin:], pb.Data)

				break waitlp

			case MsgChoke:
				c.Choked = true
				return nil, fmt.Errorf("peer has choked us")
			}
		}
	}

	return buf, nil
}

func (c *Client) GetPiece(t *torrent.Torrent, id int) (*Piece, error) {
	if id < 0 || id >= len(t.Pieces) {
		return nil, fmt.Errorf("invalid piece index %d", id)
	}

	ln := t.PieceLength
	if id == len(t.Pieces)-1 {
		rem := int(t.Length % int64(t.PieceLength))
		if rem != 0 {
			ln = rem
		}
	}

	p, err := c.downloadPiece(id, ln)
	if err != nil {
		return nil, fmt.Errorf("failed to download piece: %w", err)
	}

	hash := sha1.Sum(p)
	if hash != t.Pieces[id] {
		return nil, fmt.Errorf("piece %d failed SHA-1 verification", id)
	}

	return &Piece{ID: id, Data: p}, nil
}
