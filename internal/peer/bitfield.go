package peer

import (
	"encoding/binary"
	"fmt"
)

type Bitfield []byte

func GetBitfield(msg *Message) (Bitfield, error) {
	if msg.ID == MsgBitfield {
		return msg.Payload, nil
	}

	return nil, fmt.Errorf("message isn't a bitfield")
}

func (b Bitfield) HasPiece(pieceIndex int) bool {
	// 1B == 8b
	bitCount := len(b) * 8
	if pieceIndex >= bitCount || pieceIndex < 0 {
		return false
	}

	currentByte := b[pieceIndex/8]

	return (currentByte & (1 << (7 - (pieceIndex % 8)))) != 0
}

func (b Bitfield) SetPiece(pieceIndex int) {
	byteIndex := pieceIndex / 8
	if pieceIndex < 0 || byteIndex >= len(b) {
		return
	}

	b[byteIndex] |= 1 << (7 - (pieceIndex % 8))
}

func parseHave(msg *Message) (int, error) {
	if msg.ID != MsgHave {
		return 0, fmt.Errorf("message isn't a have")
	}

	if len(msg.Payload) != 4 {
		return 0, fmt.Errorf("invalid have payload length: %d", len(msg.Payload))
	}

	return int(binary.BigEndian.Uint32(msg.Payload)), nil
}
