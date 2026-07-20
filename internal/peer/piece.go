package peer

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"time"

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

const pipelineDepth = 5

const blockTimeout = 30 * time.Second

const blockSize = 16 * 1024

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

func (c *Client) downloadPiece(ctx context.Context, pieceIndex int, pieceLength int) ([]byte, error) {
	buf := make([]byte, pieceLength)

	var offsets []int
	for offset := 0; offset < pieceLength; offset += blockSize {
		offsets = append(offsets, offset)
	}

	next := 0
	received := 0

	requestNext := func() error {
		if next >= len(offsets) {
			return nil
		}
		offset := offsets[next]
		length := min(blockSize, pieceLength-offset)

		if err := c.Request(pieceIndex, offset, length); err != nil {
			return fmt.Errorf("failed to request block %d for piece %d: %w", offset, pieceIndex, err)
		}

		next++
		return nil
	}

	for i := 0; i < pipelineDepth && next < len(offsets); i++ {
		if err := requestNext(); err != nil {
			return nil, err
		}
	}

	for received < len(offsets) {
		if c.IsChoked() {
			return nil, fmt.Errorf("peer choked us mid-piece %d", pieceIndex)
		}

		select {
		case block, ok := <-c.blocks:
			if !ok {
				return nil, fmt.Errorf("connection closed while downloading piece %d", pieceIndex)
			}

			if block.Index != pieceIndex {
				continue
			}

			expectedLength := min(blockSize, pieceLength-block.Begin)

			if block.Begin < 0 || block.Begin+len(block.Data) > len(buf) {
				return nil, fmt.Errorf("piece block exceeds buffer")
			}

			if len(block.Data) != expectedLength {
				return nil, fmt.Errorf("expected %d bytes, got %d", expectedLength, len(block.Data))
			}

			copy(buf[block.Begin:], block.Data)
			received++

			if err := requestNext(); err != nil {
				return nil, err
			}

		case <-c.closeCh:
			return nil, fmt.Errorf("connection closed while downloading piece %d: %w", pieceIndex, c.ReadErr())

		case <-ctx.Done():
			return nil, ctx.Err()

		case <-time.After(blockTimeout):
			return nil, fmt.Errorf("timed out waiting for block in piece %d", pieceIndex)
		}
	}

	return buf, nil
}

func (c *Client) GetPiece(ctx context.Context, t *torrent.Torrent, pieceIndex int) (*Piece, error) {
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

	data, err := c.downloadPiece(ctx, pieceIndex, pieceLength)
	if err != nil {
		return nil, fmt.Errorf("failed to download piece: %w", err)
	}

	hash := sha1.Sum(data)
	if hash != t.Pieces[pieceIndex] {
		return nil, fmt.Errorf("piece %d failed SHA-1 verification", pieceIndex)
	}

	return &Piece{ID: pieceIndex, Data: data}, nil
}
