package downloader

import (
	"context"
	"crypto/sha1"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

func TestWorkerStarvationRegression(t *testing.T) {
	pieces := [][]byte{
		[]byte("AAAAAAAAAAAAAAAAAAAA"),
		[]byte("BBBBBBBBBBBBBBBBBBBB"),
	}
	var hashes [][20]byte
	var length int64
	for _, p := range pieces {
		hashes = append(hashes, sha1.Sum(p))
		length += int64(len(p))
	}

	tor := &torrent.Torrent{
		Name:        "starvation-test.bin",
		Length:      length,
		PieceLength: len(pieces[0]),
		Pieces:      hashes,
		InfoHash:    sha1.Sum([]byte("fake-info-hash-for-starvation-test")),
	}

	var mu sync.Mutex
	piece0FailedOnce := false

	handler := func(pieceIndex int) ([]byte, bool) {
		if pieceIndex == 0 {
			mu.Lock()
			shouldFail := !piece0FailedOnce
			piece0FailedOnce = true
			mu.Unlock()
			if shouldFail {
				return nil, false
			}
		}
		return pieces[pieceIndex], true
	}

	tmpDir := t.TempDir()
	peerID, err := peer.New()
	if err != nil {
		t.Fatalf("peer.New: %v", err)
	}

	var peers []tracker.Peer
	var fakeServers []*fakePeerServer
	for i := 0; i < 3; i++ {
		fp := startFakePeer(t, tor.InfoHash, allPieces(len(pieces)), handler, 0)
		defer fp.stop()
		fakeServers = append(fakeServers, fp)
		peers = append(peers, mustParsePeerAddr(t, fp.addr()))
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	d := &Downloader{
		Torrent:    tor,
		PeerID:     peerID,
		Port:       6881,
		OutputDir:  tmpDir,
		Peers:      peers,
		logger:     logger.With("component", "downloader"),
		baseLogger: logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.Download(ctx); err != nil {
		t.Fatalf("expected the download to complete despite one worker starting idle and piece 0 failing once, got: %v", err)
	}

	total := 0
	for _, fp := range fakeServers {
		total += len(fp.requestedIndices())
	}
	if total == 0 {
		t.Fatal("expected at least some requests to have been made across the three fake peers")
	}
}
