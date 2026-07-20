package tracker

import (
	"net"
	"testing"
)

func TestParseCompactPeers(t *testing.T) {
	data := string([]byte{
		192, 168, 1, 1, 0x1A, 0xE1,
		10, 0, 0, 1, 0x1A, 0xE2,
	})

	peers, err := parseCompactPeers(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(peers))
	}
	if !peers[0].IP.Equal(net.IPv4(192, 168, 1, 1)) || peers[0].Port != 0x1AE1 {
		t.Errorf("peer 0 = %+v", peers[0])
	}
	if !peers[1].IP.Equal(net.IPv4(10, 0, 0, 1)) || peers[1].Port != 0x1AE2 {
		t.Errorf("peer 1 = %+v", peers[1])
	}
}

func TestParseCompactPeersInvalidLength(t *testing.T) {
	if _, err := parseCompactPeers("12345"); err == nil {
		t.Fatal("expected error for a length not a multiple of 6")
	}
}

func TestParseCompactPeersEmpty(t *testing.T) {
	peers, err := parseCompactPeers("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected no peers, got %d", len(peers))
	}
}
