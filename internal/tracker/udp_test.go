package tracker

import (
	"context"
	"encoding/binary"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/virajsazzala/swrm/internal/torrent"
)

func startFakeUDPTracker(t *testing.T, handler func(conn *net.UDPConn, addr *net.UDPAddr, req []byte)) *net.UDPConn {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			req := make([]byte, n)
			copy(req, buf[:n])
			handler(conn, addr, req)
		}
	}()

	return conn
}

func dialUDPClient(t *testing.T, serverAddr *net.UDPAddr) *net.UDPConn {
	t.Helper()
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestUDPAnnounceWireFormat(t *testing.T) {
	const fakeConnID uint64 = 0x1122334455667788
	fakePeers := string([]byte{192, 168, 1, 1, 0x1A, 0xE1})

	var mu sync.Mutex
	var gotConnectReq, gotAnnounceReq []byte

	handler := func(conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
		action := binary.BigEndian.Uint32(req[8:12])
		txID := binary.BigEndian.Uint32(req[12:16])

		switch action {
		case udpActionConnect:
			mu.Lock()
			gotConnectReq = req
			mu.Unlock()
			resp := make([]byte, 16)
			binary.BigEndian.PutUint32(resp[0:4], udpActionConnect)
			binary.BigEndian.PutUint32(resp[4:8], txID)
			binary.BigEndian.PutUint64(resp[8:16], fakeConnID)
			conn.WriteToUDP(resp, addr)

		case udpActionAnnounce:
			mu.Lock()
			gotAnnounceReq = req
			mu.Unlock()
			resp := make([]byte, 20+len(fakePeers))
			binary.BigEndian.PutUint32(resp[0:4], udpActionAnnounce)
			binary.BigEndian.PutUint32(resp[4:8], txID)
			binary.BigEndian.PutUint32(resp[8:12], 1800)
			binary.BigEndian.PutUint32(resp[12:16], 5)
			binary.BigEndian.PutUint32(resp[16:20], 10)
			copy(resp[20:], fakePeers)
			conn.WriteToUDP(resp, addr)
		}
	}

	serverConn := startFakeUDPTracker(t, handler)
	serverAddr := serverConn.LocalAddr().(*net.UDPAddr)

	u, err := url.Parse("udp://" + serverAddr.String())
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	tor := &torrent.Torrent{Length: 999}
	copy(tor.InfoHash[:], "01234567890123456789")
	peerID := testPeerID()

	resp, err := announceUDP(context.Background(), discardLogger(), u, tor, peerID, 6881, EventStarted)
	if err != nil {
		t.Fatalf("announceUDP failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(gotConnectReq) != 16 {
		t.Fatalf("connect request length = %d, want 16", len(gotConnectReq))
	}
	if magic := binary.BigEndian.Uint64(gotConnectReq[0:8]); magic != udpProtocolMagic {
		t.Errorf("connect magic = %#x, want %#x", magic, uint64(udpProtocolMagic))
	}

	if len(gotAnnounceReq) != 98 {
		t.Fatalf("announce request length = %d, want 98", len(gotAnnounceReq))
	}
	if gotConnID := binary.BigEndian.Uint64(gotAnnounceReq[0:8]); gotConnID != fakeConnID {
		t.Errorf("announce connection_id = %#x, want %#x", gotConnID, fakeConnID)
	}
	var gotInfoHash [20]byte
	copy(gotInfoHash[:], gotAnnounceReq[16:36])
	if gotInfoHash != tor.InfoHash {
		t.Errorf("announce info_hash mismatch: got %x, want %x", gotInfoHash, tor.InfoHash)
	}
	var gotPeerID [20]byte
	copy(gotPeerID[:], gotAnnounceReq[36:56])
	if gotPeerID != peerID {
		t.Errorf("announce peer_id mismatch: got %x, want %x", gotPeerID, peerID)
	}
	if gotLeft := binary.BigEndian.Uint64(gotAnnounceReq[64:72]); gotLeft != uint64(tor.Length) {
		t.Errorf("announce left = %d, want %d", gotLeft, tor.Length)
	}
	if gotEvent := binary.BigEndian.Uint32(gotAnnounceReq[80:84]); gotEvent != EventStarted.udpValue() {
		t.Errorf("announce event = %d, want %d", gotEvent, EventStarted.udpValue())
	}
	if gotPort := binary.BigEndian.Uint16(gotAnnounceReq[96:98]); gotPort != 6881 {
		t.Errorf("announce port = %d, want 6881", gotPort)
	}

	if resp.Interval != 1800 {
		t.Errorf("Interval = %d, want 1800", resp.Interval)
	}
	if len(resp.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(resp.Peers))
	}
}

func TestUDPConnectRetriesOnShortResponse(t *testing.T) {
	attempts := make(map[uint32]int)

	handler := func(conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
		action := binary.BigEndian.Uint32(req[8:12])
		if action != udpActionConnect {
			return
		}
		txID := binary.BigEndian.Uint32(req[12:16])

		attempts[txID]++
		if attempts[txID] == 1 {
			conn.WriteToUDP([]byte{1, 2, 3}, addr)
			return
		}

		resp := make([]byte, 16)
		binary.BigEndian.PutUint32(resp[0:4], udpActionConnect)
		binary.BigEndian.PutUint32(resp[4:8], txID)
		binary.BigEndian.PutUint64(resp[8:16], 0xAABBCCDD)
		conn.WriteToUDP(resp, addr)
	}

	serverConn := startFakeUDPTracker(t, handler)
	client := dialUDPClient(t, serverConn.LocalAddr().(*net.UDPAddr))

	connID, err := udpConnect(context.Background(), discardLogger(), client)
	if err != nil {
		t.Fatalf("udpConnect failed after retry: %v", err)
	}
	if connID != 0xAABBCCDD {
		t.Errorf("connID = %#x, want %#x", connID, uint64(0xAABBCCDD))
	}
}

func TestUDPConnectTransactionIDMismatch(t *testing.T) {
	handler := func(conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
		resp := make([]byte, 16)
		binary.BigEndian.PutUint32(resp[0:4], udpActionConnect)
		binary.BigEndian.PutUint32(resp[4:8], 0xDEADBEEF)
		binary.BigEndian.PutUint64(resp[8:16], 123)
		conn.WriteToUDP(resp, addr)
	}

	serverConn := startFakeUDPTracker(t, handler)
	client := dialUDPClient(t, serverConn.LocalAddr().(*net.UDPAddr))

	if _, err := udpConnect(context.Background(), discardLogger(), client); err == nil {
		t.Fatal("expected a transaction id mismatch error")
	}
}

func TestUDPConnectTrackerError(t *testing.T) {
	handler := func(conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
		txID := binary.BigEndian.Uint32(req[12:16])
		errMsg := "bad torrent"
		resp := make([]byte, 8+len(errMsg))
		binary.BigEndian.PutUint32(resp[0:4], udpActionError)
		binary.BigEndian.PutUint32(resp[4:8], txID)
		copy(resp[8:], errMsg)
		conn.WriteToUDP(resp, addr)
	}

	serverConn := startFakeUDPTracker(t, handler)
	client := dialUDPClient(t, serverConn.LocalAddr().(*net.UDPAddr))

	_, err := udpConnect(context.Background(), discardLogger(), client)
	if err == nil || !strings.Contains(err.Error(), "bad torrent") {
		t.Fatalf("expected the tracker error message to be surfaced, got: %v", err)
	}
}

func TestUDPAnnounceCancellationIsPrompt(t *testing.T) {
	handler := func(conn *net.UDPConn, addr *net.UDPAddr, req []byte) {}
	serverConn := startFakeUDPTracker(t, handler)
	serverAddr := serverConn.LocalAddr().(*net.UDPAddr)

	u, err := url.Parse("udp://" + serverAddr.String())
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	tor := &torrent.Torrent{Length: 100}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = announceUDP(ctx, discardLogger(), u, tor, testPeerID(), 6881, EventNone)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error from the cancelled context")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("cancellation took too long to take effect: %v", elapsed)
	}
}
