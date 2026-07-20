package peer

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	writer := &Client{Conn: clientConn}
	reader := &Client{Conn: serverConn}

	msg := &Message{ID: MsgPiece, Payload: []byte("hello")}

	done := make(chan error, 1)
	go func() { done <- writer.WriteMessage(msg) }()

	got, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	if got.ID != msg.ID || string(got.Payload) != string(msg.Payload) {
		t.Fatalf("got %+v, want %+v", got, msg)
	}
}

func TestMessageKeepAlive(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	writer := &Client{Conn: clientConn}
	reader := &Client{Conn: serverConn}

	done := make(chan error, 1)
	go func() { done <- writer.WriteMessage(&Message{KeepAlive: true}) }()

	got, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if !got.KeepAlive {
		t.Fatal("expected KeepAlive message")
	}
	if err := <-done; err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

func TestMessageExceedsMaxLength(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	reader := &Client{Conn: serverConn}

	go func() {
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], maxMessageLength+1)
		clientConn.Write(lenBuf[:])
	}()

	if _, err := reader.ReadMessage(); err == nil {
		t.Fatal("expected error for a message length exceeding the max")
	}
}

func TestMessageEmptyPayloadRoundTrip(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	writer := &Client{Conn: clientConn}
	reader := &Client{Conn: serverConn}

	done := make(chan error, 1)
	go func() { done <- writer.WriteMessage(&Message{ID: MsgUnchoke}) }()

	got, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	if got.ID != MsgUnchoke || len(got.Payload) != 0 {
		t.Fatalf("got %+v, want ID=%d with empty payload", got, MsgUnchoke)
	}
}
