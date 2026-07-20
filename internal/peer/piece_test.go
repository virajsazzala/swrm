package peer

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func newTestClientPair(t *testing.T, pieceCount int) (*Client, net.Conn) {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		serverConn.Close()
		clientConn.Close()
	})

	c := &Client{
		Conn:    clientConn,
		closeCh: make(chan struct{}),
		blocks:  make(chan *Block, 4*pipelineDepth),
		notify:  make(chan struct{}, 1),
	}
	c.Start(pieceCount)

	go io.Copy(io.Discard, serverConn)

	return c, serverConn
}

func sendPieceMessage(t *testing.T, conn net.Conn, index, begin int, data []byte) {
	t.Helper()

	payload := make([]byte, 8+len(data))
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	copy(payload[8:], data)

	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(1+len(payload)))

	conn.Write(lenBuf)
	conn.Write([]byte{MsgPiece})
	conn.Write(payload)
}

type pieceResult struct {
	data []byte
	err  error
}

func TestDownloadPieceHappyPath(t *testing.T) {
	pieceLength := blockSize + 10
	c, serverConn := newTestClientPair(t, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	block0 := bytes.Repeat([]byte{0xAA}, blockSize)
	block1 := bytes.Repeat([]byte{0xBB}, 10)

	resultCh := make(chan pieceResult, 1)
	go func() {
		data, err := c.downloadPiece(ctx, 0, pieceLength)
		resultCh <- pieceResult{data, err}
	}()

	sendPieceMessage(t, serverConn, 0, 0, block0)
	sendPieceMessage(t, serverConn, 0, blockSize, block1)

	res := <-resultCh
	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}

	want := append(append([]byte{}, block0...), block1...)
	if !bytes.Equal(res.data, want) {
		t.Fatal("downloaded data doesn't match what was sent")
	}
}

func TestDownloadPieceRejectsUnrequestedOffset(t *testing.T) {
	pieceLength := blockSize + 10
	c, serverConn := newTestClientPair(t, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultCh := make(chan pieceResult, 1)
	go func() {
		data, err := c.downloadPiece(ctx, 0, pieceLength)
		resultCh <- pieceResult{data, err}
	}()

	sendPieceMessage(t, serverConn, 0, 5, make([]byte, blockSize))

	res := <-resultCh
	if res.err == nil {
		t.Fatal("expected error for a block at an offset that was never requested")
	}
}

func TestDownloadPieceDuplicateBlockDoesNotFakeCompletion(t *testing.T) {
	pieceLength := blockSize + 10
	c, serverConn := newTestClientPair(t, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	resultCh := make(chan pieceResult, 1)
	go func() {
		data, err := c.downloadPiece(ctx, 0, pieceLength)
		resultCh <- pieceResult{data, err}
	}()

	for i := 0; i < 5; i++ {
		sendPieceMessage(t, serverConn, 0, 0, bytes.Repeat([]byte{0xCC}, blockSize))
	}

	res := <-resultCh
	if res.err == nil {
		t.Fatal("expected the piece to time out since the second block (offset=blockSize) was never actually delivered, only duplicates of the first")
	}
}

func TestDownloadPieceRejectsOversizedBlock(t *testing.T) {
	pieceLength := blockSize
	c, serverConn := newTestClientPair(t, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultCh := make(chan pieceResult, 1)
	go func() {
		data, err := c.downloadPiece(ctx, 0, pieceLength)
		resultCh <- pieceResult{data, err}
	}()

	sendPieceMessage(t, serverConn, 0, 0, make([]byte, blockSize+100))

	res := <-resultCh
	if res.err == nil {
		t.Fatal("expected error for an oversized block")
	}
}

func TestDownloadPieceIgnoresWrongPieceIndex(t *testing.T) {
	pieceLength := blockSize
	c, serverConn := newTestClientPair(t, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultCh := make(chan pieceResult, 1)
	go func() {
		data, err := c.downloadPiece(ctx, 0, pieceLength)
		resultCh <- pieceResult{data, err}
	}()

	block := bytes.Repeat([]byte{0xDD}, blockSize)
	sendPieceMessage(t, serverConn, 99, 0, block)
	sendPieceMessage(t, serverConn, 0, 0, block)

	res := <-resultCh
	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if !bytes.Equal(res.data, block) {
		t.Fatal("downloaded data doesn't match the block sent for the correct piece index")
	}
}
