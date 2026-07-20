package peer

import (
	"net"
	"strings"
	"testing"
)

func newTCPLoopbackPair(t *testing.T) (server, client net.Conn) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	acceptCh := make(chan net.Conn, 1)
	acceptErrCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			acceptErrCh <- err
			return
		}
		acceptCh <- conn
	}()

	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	select {
	case server = <-acceptCh:
	case err := <-acceptErrCh:
		t.Fatalf("accept: %v", err)
	}

	t.Cleanup(func() {
		server.Close()
		client.Close()
	})

	return server, client
}

func TestHandshakeRoundTrip(t *testing.T) {
	serverConn, clientConn := newTCPLoopbackPair(t)

	var infoHash [20]byte
	copy(infoHash[:], "01234567890123456789")
	var peerIDA [20]byte
	copy(peerIDA[:], "AAAAAAAAAAAAAAAAAAAA")
	var peerIDB [20]byte
	copy(peerIDB[:], "BBBBBBBBBBBBBBBBBBBB")

	clientSide := &Client{Conn: clientConn}
	serverSide := &Client{Conn: serverConn}

	errCh := make(chan error, 1)
	go func() { errCh <- serverSide.Handshake(infoHash, peerIDB) }()

	if err := clientSide.Handshake(infoHash, peerIDA); err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("server handshake: %v", err)
	}

	if clientSide.PeerID != peerIDB {
		t.Errorf("client recorded wrong peer id: got %x, want %x", clientSide.PeerID, peerIDB)
	}
	if serverSide.PeerID != peerIDA {
		t.Errorf("server recorded wrong peer id: got %x, want %x", serverSide.PeerID, peerIDA)
	}
}

func TestHandshakeInfoHashMismatch(t *testing.T) {
	serverConn, clientConn := newTCPLoopbackPair(t)

	var hashA, hashB [20]byte
	copy(hashA[:], "AAAAAAAAAAAAAAAAAAAA")
	copy(hashB[:], "BBBBBBBBBBBBBBBBBBBB")
	var peerID [20]byte

	clientSide := &Client{Conn: clientConn}
	serverSide := &Client{Conn: serverConn}

	errCh := make(chan error, 1)
	go func() { errCh <- serverSide.Handshake(hashB, peerID) }()

	err := clientSide.Handshake(hashA, peerID)
	<-errCh

	if err == nil {
		t.Fatal("expected info_hash mismatch to be rejected")
	}
	if !strings.Contains(err.Error(), "info_hash") {
		t.Fatalf("expected an info_hash related error, got: %v", err)
	}
}
