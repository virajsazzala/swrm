package peer

import (
	"encoding/binary"
	"testing"
)

func TestBitfieldHasPiece(t *testing.T) {
	bf := Bitfield{0b10000000, 0b00000001}

	cases := []struct {
		index int
		want  bool
	}{
		{0, true},
		{1, false},
		{7, false},
		{8, false},
		{15, true},
		{16, false},
		{-1, false},
	}

	for _, c := range cases {
		if got := bf.HasPiece(c.index); got != c.want {
			t.Errorf("HasPiece(%d) = %v, want %v", c.index, got, c.want)
		}
	}
}

func TestBitfieldSetPiece(t *testing.T) {
	bf := make(Bitfield, 2)
	bf.SetPiece(0)
	bf.SetPiece(15)
	bf.SetPiece(100)
	bf.SetPiece(-1)

	if !bf.HasPiece(0) || !bf.HasPiece(15) {
		t.Fatalf("expected pieces 0 and 15 set, got %08b %08b", bf[0], bf[1])
	}
	if bf.HasPiece(1) || bf.HasPiece(14) {
		t.Fatalf("unexpected extra bits set: %08b %08b", bf[0], bf[1])
	}
}

func TestGetBitfield(t *testing.T) {
	msg := &Message{ID: MsgBitfield, Payload: []byte{0xFF}}
	bf, err := GetBitfield(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bf.HasPiece(0) {
		t.Fatal("expected piece 0 set")
	}

	if _, err := GetBitfield(&Message{ID: MsgChoke}); err == nil {
		t.Fatal("expected error for non-bitfield message")
	}
}

func TestParseHave(t *testing.T) {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, 42)

	idx, err := parseHave(&Message{ID: MsgHave, Payload: payload})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 42 {
		t.Errorf("got %d, want 42", idx)
	}

	if _, err := parseHave(&Message{ID: MsgHave, Payload: []byte{1, 2, 3}}); err == nil {
		t.Fatal("expected error for wrong payload length")
	}

	if _, err := parseHave(&Message{ID: MsgChoke, Payload: payload}); err == nil {
		t.Fatal("expected error for wrong message id")
	}
}
