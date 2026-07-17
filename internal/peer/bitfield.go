package peer

import "fmt"

type Bitfield []byte

func GetBitfield(msg *Message) (Bitfield, error) {
	if msg.ID == MsgBitfield {
		return msg.Payload, nil
	}

	return nil, fmt.Errorf("message isn't a bitfield")
}

func (b Bitfield) HasPiece(idx int) bool {
	// 1B == 8b
	nb := len(b) * 8
	if idx >= nb || idx < 0 {
		return false
	}

	cb := b[idx/8]

	return (cb & (1 << (7 - (idx % 8)))) != 0
}
