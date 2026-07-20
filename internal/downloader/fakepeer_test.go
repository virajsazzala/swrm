package downloader

import (
	"crypto/sha1"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

const (
	fakeMsgUnchoke    = 1
	fakeMsgInterested = 2
	fakeMsgBitfield   = 5
	fakeMsgRequest    = 6
	fakeMsgPiece      = 7
)

func fakeWriteMsg(conn net.Conn, id byte, payload []byte) error {
	buf := make([]byte, 4+1+len(payload))
	binary.BigEndian.PutUint32(buf[0:4], uint32(1+len(payload)))
	buf[4] = id
	copy(buf[5:], payload)
	_, err := conn.Write(buf)
	return err
}

type fakeMsg struct {
	id      byte
	payload []byte
}

func fakeReadMsg(conn net.Conn) (*fakeMsg, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	if length == 0 {
		return nil, nil
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}
	return &fakeMsg{id: data[0], payload: data[1:]}, nil
}

type fakePieceHandler func(pieceIndex int) (data []byte, ok bool)

type fakePeerServer struct {
	ln         net.Listener
	infoHash   [20]byte
	bitfieldOf func() []int
	handler    fakePieceHandler
	delay      time.Duration
	served     chan int

	mu        sync.Mutex
	requested []int
}

func startFakePeer(t *testing.T, infoHash [20]byte, havePieces []int, handler fakePieceHandler, delay time.Duration) *fakePeerServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	fp := &fakePeerServer{
		ln:       ln,
		infoHash: infoHash,
		handler:  handler,
		delay:    delay,
		served:   make(chan int, 4096),
	}
	t.Cleanup(fp.stop)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go fp.serve(conn, havePieces)
		}
	}()

	return fp
}

func (fp *fakePeerServer) addr() string { return fp.ln.Addr().String() }
func (fp *fakePeerServer) stop()        { fp.ln.Close() }

func (fp *fakePeerServer) requestedIndices() []int {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	return append([]int(nil), fp.requested...)
}

func (fp *fakePeerServer) serve(conn net.Conn, havePieces []int) {
	defer conn.Close()

	var hs [68]byte
	if _, err := io.ReadFull(conn, hs[:]); err != nil {
		return
	}

	var resp [68]byte
	resp[0] = 19
	copy(resp[1:20], "BitTorrent protocol")
	copy(resp[28:48], fp.infoHash[:])
	copy(resp[48:68], []byte("-FK0001-000000000000"))
	if _, err := conn.Write(resp[:]); err != nil {
		return
	}

	maxPiece := 0
	for _, p := range havePieces {
		if p > maxPiece {
			maxPiece = p
		}
	}
	bf := make([]byte, maxPiece/8+1)
	for _, p := range havePieces {
		bf[p/8] |= 1 << (7 - (p % 8))
	}
	if err := fakeWriteMsg(conn, fakeMsgBitfield, bf); err != nil {
		return
	}

	for {
		msg, err := fakeReadMsg(conn)
		if err != nil {
			return
		}
		if msg == nil {
			continue
		}

		switch msg.id {
		case fakeMsgInterested:
			if err := fakeWriteMsg(conn, fakeMsgUnchoke, nil); err != nil {
				return
			}

		case fakeMsgRequest:
			index := int(binary.BigEndian.Uint32(msg.payload[0:4]))
			begin := binary.BigEndian.Uint32(msg.payload[4:8])
			length := binary.BigEndian.Uint32(msg.payload[8:12])

			fp.mu.Lock()
			fp.requested = append(fp.requested, index)
			fp.mu.Unlock()

			if fp.delay > 0 {
				time.Sleep(fp.delay)
			}

			data, ok := fp.handler(index)
			if !ok {
				return
			}

			block := data[begin : begin+length]
			payload := make([]byte, 8+len(block))
			binary.BigEndian.PutUint32(payload[0:4], uint32(index))
			binary.BigEndian.PutUint32(payload[4:8], begin)
			copy(payload[8:], block)
			if err := fakeWriteMsg(conn, fakeMsgPiece, payload); err != nil {
				return
			}

			fp.served <- index
		}
	}
}

func mustParsePeerAddr(t *testing.T, addr string) tracker.Peer {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return tracker.Peer{IP: net.ParseIP(host), Port: uint16(port)}
}

func alwaysServe(pieces [][]byte) fakePieceHandler {
	return func(pieceIndex int) ([]byte, bool) {
		if pieceIndex < 0 || pieceIndex >= len(pieces) {
			return nil, false
		}
		return pieces[pieceIndex], true
	}
}

func verifyPiecesOnDiskSingleFile(t *testing.T, path string, expected [][]byte) []int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	var complete []int
	offset := 0
	for i, want := range expected {
		if offset+len(want) > len(data) {
			continue
		}
		got := data[offset : offset+len(want)]
		if sha1.Sum(got) == sha1.Sum(want) {
			complete = append(complete, i)
		}
		offset += len(want)
	}
	return complete
}

func verifyPiecesOnDiskMultiFile(t *testing.T, dir string, tor *torrent.Torrent, expected [][]byte) []int {
	t.Helper()

	var full []byte
	for _, fi := range tor.Files {
		path := filepath.Join(append([]string{dir, tor.Name}, fi.Path...)...)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		full = append(full, data...)
	}

	var complete []int
	offset := 0
	for i, want := range expected {
		if offset+len(want) > len(full) {
			continue
		}
		got := full[offset : offset+len(want)]
		if sha1.Sum(got) == sha1.Sum(want) {
			complete = append(complete, i)
		}
		offset += len(want)
	}
	return complete
}
