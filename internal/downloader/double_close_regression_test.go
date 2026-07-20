package downloader

import (
	"context"
	"crypto/sha1"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

func TestDoubleCloseOnStaleWorkersAfterReconnectFailure(t *testing.T) {
	pieces := [][]byte{[]byte("AAAAAAAAAAAAAAAAAAAA")}
	hash := sha1.Sum(pieces[0])

	tor := &torrent.Torrent{
		Name:        "repro.bin",
		Length:      int64(len(pieces[0])),
		PieceLength: len(pieces[0]),
		Pieces:      [][20]byte{hash},
		InfoHash:    sha1.Sum([]byte("repro-info-hash")),
	}

	fp := startFakePeer(t, tor.InfoHash, []int{}, alwaysServe(pieces), 0)
	defer fp.stop()

	peerID, err := peer.New()
	if err != nil {
		t.Fatalf("peer.New: %v", err)
	}
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	d := &Downloader{
		Torrent:    tor,
		PeerID:     peerID,
		Port:       6881,
		OutputDir:  tmpDir,
		Peers:      []tracker.Peer{mustParsePeerAddr(t, fp.addr())},
		logger:     logger.With("component", "downloader"),
		baseLogger: logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err = d.Download(ctx)
	if err == nil {
		t.Fatal("expected an error")
	}
	t.Logf("Download returned (no panic): %v", err)
}
