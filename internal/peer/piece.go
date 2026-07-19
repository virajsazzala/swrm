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

func (c *Client) downloadPiece(pieceIndex int, pieceLength int) ([]byte, error) {
	buf := make([]byte, pieceLength)
	const blockSize = 16 * 1024

	for offset := 0; offset < pieceLength; offset += blockSize {
		blockLength := min(blockSize, pieceLength-offset)

		if err := c.Request(pieceIndex, offset, blockLength); err != nil {
			return nil, fmt.Errorf("failed to get block %v for piece %v", offset, pieceIndex)
		}

	waitlp:
		for {
			msg, err := c.ReadMessage()
			if err != nil {
				return nil, fmt.Errorf("failed to read piece %v, block %v: %v\n", pieceIndex, offset, err)
			}

			if msg.KeepAlive {
				continue
			}

			switch msg.ID {
			case MsgPiece:
				block, err := parseBlock(msg)
				if err != nil {
					return nil, fmt.Errorf("failed to parse block for piece %d: %v", pieceIndex, err)
				}

				if block.Index != pieceIndex {
					return nil, fmt.Errorf("invalid piece block, request %v, got %v", pieceIndex, block.Index)
				}

				if block.Begin != offset {
					return nil, fmt.Errorf("invalid piece block offset, request %v, got %v", offset, block.Begin)
				}

				if block.Begin+len(block.Data) > len(buf) {
					return nil, fmt.Errorf("piece block exceeds buffer")
				}

				if len(block.Data) != blockLength {
					return nil, fmt.Errorf("expected %d bytes, got %d", blockLength, len(block.Data))
				}

				copy(buf[block.Begin:], block.Data)

				break waitlp

			case MsgChoke:
				c.Choked = true
				return nil, fmt.Errorf("peer has choked us")
			}
		}
	}

	return buf, nil
}

func (c *Client) GetPiece(t *torrent.Torrent, pieceIndex int) (*Piece, error) {
	if pieceIndex < 0 || pieceIndex >= len(t.Pieces) {
		return nil, fmt.Errorf("invalid piece index %d", pieceIndex)
	}

	pieceLength := t.PieceLength
	if pieceIndex == len(t.Pieces)-1 {
		rem := int(t.Length % int64(t.PieceLength))
		if rem != 0 {
			pieceLength = rem
		}
	}

	data, err := c.downloadPiece(pieceIndex, pieceLength)
	if err != nil {
		return nil, fmt.Errorf("failed to download piece: %w", err)
	}

	hash := sha1.Sum(data)
	if hash != t.Pieces[pieceIndex] {
		return nil, fmt.Errorf("piece %d failed SHA-1 verification", pieceIndex)
	}

	return &Piece{ID: pieceIndex, Data: data}, nil
}
