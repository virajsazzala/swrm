package peer

import "fmt"

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
