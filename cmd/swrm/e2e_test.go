package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/virajsazzala/swrm/internal/api"
)

const (
	fakeMsgUnchoke    = 1
	fakeMsgInterested = 2
	fakeMsgBitfield   = 5
	fakeMsgRequest    = 6
	fakeMsgPiece      = 7
)

func bStr(s string) string { return fmt.Sprintf("%d:%s", len(s), s) }
func bInt(n int64) string  { return fmt.Sprintf("i%de", n) }

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func buildBinary(t *testing.T, binDir, name, pkg string) string {
	t.Helper()
	out := filepath.Join(binDir, name)
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = moduleRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, output)
	}
	return out
}

func compactPeer(t *testing.T, addr string) string {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split addr %q: %v", addr, err)
	}
	ip := net.ParseIP(host).To4()
	if ip == nil {
		t.Fatalf("not an IPv4 address: %q", host)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}
	buf := make([]byte, 6)
	copy(buf[0:4], ip)
	binary.BigEndian.PutUint16(buf[4:6], uint16(port))
	return string(buf)
}

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

func startFakePeer(t *testing.T, infoHash [20]byte, pieces [][]byte) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFakePeer(conn, infoHash, pieces)
		}
	}()

	return ln.Addr().String()
}

func serveFakePeer(conn net.Conn, infoHash [20]byte, pieces [][]byte) {
	defer conn.Close()

	var hs [68]byte
	if _, err := io.ReadFull(conn, hs[:]); err != nil {
		return
	}

	var resp [68]byte
	resp[0] = 19
	copy(resp[1:20], "BitTorrent protocol")
	copy(resp[28:48], infoHash[:])
	copy(resp[48:68], bytes.Repeat([]byte{'F'}, 20))
	if _, err := conn.Write(resp[:]); err != nil {
		return
	}

	bf := make([]byte, len(pieces)/8+1)
	for i := range pieces {
		bf[i/8] |= 1 << (7 - (i % 8))
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
			index := binary.BigEndian.Uint32(msg.payload[0:4])
			begin := binary.BigEndian.Uint32(msg.payload[4:8])
			length := binary.BigEndian.Uint32(msg.payload[8:12])

			if int(index) >= len(pieces) {
				return
			}
			data := pieces[index]
			if int(begin+length) > len(data) {
				return
			}
			block := data[begin : begin+length]

			payload := make([]byte, 8+len(block))
			binary.BigEndian.PutUint32(payload[0:4], index)
			binary.BigEndian.PutUint32(payload[4:8], begin)
			copy(payload[8:], block)
			if err := fakeWriteMsg(conn, fakeMsgPiece, payload); err != nil {
				return
			}
		}
	}
}

func writeTestTorrent(t *testing.T, trackerURL string, pieces [][]byte, pieceLength int) (path string, infoHash [20]byte) {
	t.Helper()

	var piecesConcat strings.Builder
	var totalLength int64
	for _, p := range pieces {
		h := sha1.Sum(p)
		piecesConcat.Write(h[:])
		totalLength += int64(len(p))
	}

	infoDict := "d" +
		bStr("length") + bInt(totalLength) +
		bStr("name") + bStr("testfile.bin") +
		bStr("piece length") + bInt(int64(pieceLength)) +
		bStr("pieces") + bStr(piecesConcat.String()) +
		"e"
	infoHash = sha1.Sum([]byte(infoDict))

	torrentBytes := "d" +
		bStr("announce") + bStr(trackerURL) +
		bStr("info") + infoDict +
		"e"

	dir := t.TempDir()
	path = filepath.Join(dir, "test.torrent")
	if err := os.WriteFile(path, []byte(torrentBytes), 0644); err != nil {
		t.Fatalf("write torrent file: %v", err)
	}
	return path, infoHash
}

func TestSwrmEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	binDir := t.TempDir()
	swrmPath := buildBinary(t, binDir, "swrm", "./cmd/swrm")
	buildBinary(t, binDir, "swrmd", "./cmd/swrmd")

	pieceLength := 16
	pieces := [][]byte{
		bytes.Repeat([]byte{'A'}, pieceLength),
		bytes.Repeat([]byte{'B'}, pieceLength),
	}

	var peerCompact atomic.Value

	trackerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := "d" + bStr("interval") + bInt(1800) + bStr("peers") + bStr(peerCompact.Load().(string)) + "e"
		w.Write([]byte(resp))
	}))
	t.Cleanup(trackerSrv.Close)

	torrentPath, infoHash := writeTestTorrent(t, trackerSrv.URL, pieces, pieceLength)

	peerAddr := startFakePeer(t, infoHash, pieces)
	peerCompact.Store(compactPeer(t, peerAddr))

	outputDir := t.TempDir()
	socketDir := t.TempDir()
	socketPath := filepath.Join(socketDir, "swrmd.sock")

	startOut, err := exec.Command(swrmPath, "start", torrentPath, "-output-dir", outputDir, "-socket", socketPath, "-d").CombinedOutput()
	if err != nil {
		t.Fatalf("swrm start -d: %v\n%s", err, startOut)
	}
	if !strings.Contains(string(startOut), "started swrmd") {
		t.Fatalf("unexpected start output: %s", startOut)
	}
	t.Cleanup(func() {
		exec.Command(swrmPath, "stop", "-socket", socketPath).Run()
	})

	dupOut, dupErr := exec.Command(swrmPath, "start", torrentPath, "-socket", socketPath, "-d").CombinedOutput()
	if dupErr == nil {
		t.Fatalf("expected a second start on the same socket to fail, got: %s", dupOut)
	}
	if !strings.Contains(string(dupOut), "already running") {
		t.Fatalf("expected an 'already running' message, got: %s", dupOut)
	}

	client := api.NewClient(socketPath)
	var final *api.StatusResponse
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		status, statusErr := client.Status(ctx)
		cancel()
		if statusErr == nil && (status.Status == "completed" || status.Status == "error") {
			final = status
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if final == nil {
		t.Fatal("download did not reach a terminal state in time")
	}
	if final.Status != "completed" {
		t.Fatalf("expected status completed, got %q (last error: %q)", final.Status, final.LastError)
	}
	if final.Completed != len(pieces) || final.Total != len(pieces) {
		t.Fatalf("expected %d/%d pieces, got %d/%d", len(pieces), len(pieces), final.Completed, final.Total)
	}

	got, err := os.ReadFile(filepath.Join(outputDir, "testfile.bin"))
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	var want []byte
	for _, p := range pieces {
		want = append(want, p...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("downloaded content mismatch: got %d bytes, want %d bytes", len(got), len(want))
	}

	statusOut, err := exec.Command(swrmPath, "status", "-socket", socketPath).CombinedOutput()
	if err != nil {
		t.Fatalf("swrm status: %v\n%s", err, statusOut)
	}
	if !strings.Contains(string(statusOut), "completed") {
		t.Fatalf("swrm status output missing completed status: %s", statusOut)
	}
	if !strings.Contains(string(statusOut), "testfile.bin") {
		t.Fatalf("swrm status output missing torrent name: %s", statusOut)
	}

	listOut, err := exec.Command(swrmPath, "list", "-dir", socketDir).CombinedOutput()
	if err != nil {
		t.Fatalf("swrm list: %v\n%s", err, listOut)
	}
	if !strings.Contains(string(listOut), "testfile.bin") || !strings.Contains(string(listOut), "completed") {
		t.Fatalf("swrm list output missing expected info: %s", listOut)
	}

	stopOut, err := exec.Command(swrmPath, "stop", "-socket", socketPath).CombinedOutput()
	if err != nil {
		t.Fatalf("swrm stop: %v\n%s", err, stopOut)
	}

	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(socketPath); os.IsNotExist(statErr) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if _, statErr := os.Stat(socketPath); statErr == nil {
		t.Fatal("expected the socket file to be removed after stop")
	}
}

func TestSwrmListCleanRemovesStaleSocket(t *testing.T) {
	binDir := t.TempDir()
	swrmPath := buildBinary(t, binDir, "swrm", "./cmd/swrm")

	dir := t.TempDir()
	stalePath := filepath.Join(dir, "stale.sock")

	if err := os.WriteFile(stalePath, nil, 0644); err != nil {
		t.Fatalf("write stale socket placeholder: %v", err)
	}

	out, err := exec.Command(swrmPath, "list", "-dir", dir, "-clean").CombinedOutput()
	if err != nil {
		t.Fatalf("swrm list -clean: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "removed stale socket") {
		t.Fatalf("expected removal confirmation, got: %s", out)
	}

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale socket file to be removed, stat err = %v", err)
	}
}
