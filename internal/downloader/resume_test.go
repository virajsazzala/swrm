package downloader

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

func allPieces(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return p
}

func runInterruptedRound(t *testing.T, tor *torrent.Torrent, peerID [20]byte, outputDir string, pieces [][]byte, logger *slog.Logger, stopAfterServed int) *fakePeerServer {
	t.Helper()

	fp := startFakePeer(t, tor.InfoHash, allPieces(len(pieces)), alwaysServe(pieces), 0)

	d := &Downloader{
		Torrent:    tor,
		PeerID:     peerID,
		Port:       6881,
		OutputDir:  outputDir,
		Peers:      []tracker.Peer{mustParsePeerAddr(t, fp.addr())},
		logger:     logger.With("component", "downloader"),
		baseLogger: logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		served := 0
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-fp.served:
				if !ok {
					return
				}
				served++
				if served >= stopAfterServed {
					cancel()
					return
				}
			}
		}
	}()

	err := d.Download(ctx)
	if err == nil {
		t.Fatal("expected the interrupted round to return an error, got nil")
	}

	return fp
}

func TestResumeEndToEndSingleFile(t *testing.T) {
	tmpDir := t.TempDir()

	pieces := [][]byte{
		[]byte("AAAAAAAAAAAAAAAAAAAA"),
		[]byte("BBBBBBBBBBBBBBBBBBBB"),
		[]byte("CCCCCCCCCCCCCCCCCCCC"),
		[]byte("DDDDDDDDDDDDDDDDDDDD"),
	}

	var hashes [][20]byte
	var length int64
	for _, p := range pieces {
		hashes = append(hashes, sha1.Sum(p))
		length += int64(len(p))
	}

	tor := &torrent.Torrent{
		Name:        "e2e-resume-test.bin",
		Length:      length,
		PieceLength: len(pieces[0]),
		Pieces:      hashes,
		InfoHash:    sha1.Sum([]byte("fake-info-hash-for-e2e-resume-test")),
	}

	peerID, err := peer.New()
	if err != nil {
		t.Fatalf("peer.New: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	fp1 := runInterruptedRound(t, tor, peerID, tmpDir, pieces, logger, 2)
	defer fp1.stop()

	outputPath := filepath.Join(tmpDir, tor.Name)
	completeAfterRound1 := verifyPiecesOnDiskSingleFile(t, outputPath, pieces)
	if len(completeAfterRound1) == 0 || len(completeAfterRound1) == len(pieces) {
		t.Fatalf("expected partial completion after round 1, got %v", completeAfterRound1)
	}

	fp2 := startFakePeer(t, tor.InfoHash, allPieces(len(pieces)), alwaysServe(pieces), 0)
	defer fp2.stop()

	d2 := &Downloader{
		Torrent:    tor,
		PeerID:     peerID,
		Port:       6881,
		OutputDir:  tmpDir,
		Peers:      []tracker.Peer{mustParsePeerAddr(t, fp2.addr())},
		logger:     logger.With("component", "downloader"),
		baseLogger: logger,
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	if err := d2.Download(ctx2); err != nil {
		t.Fatalf("round 2 (resume) failed: %v", err)
	}

	requestedRound2 := fp2.requestedIndices()
	for _, idx := range completeAfterRound1 {
		for _, r := range requestedRound2 {
			if r == idx {
				t.Fatalf("piece %d was already correct on disk after round 1 but was re-requested from the network in round 2", idx)
			}
		}
	}

	finalComplete := verifyPiecesOnDiskSingleFile(t, outputPath, pieces)
	if len(finalComplete) != len(pieces) {
		t.Fatalf("expected all %d pieces present after round 2, got %d: %v", len(pieces), len(finalComplete), finalComplete)
	}
}

func TestResumeEndToEndMultiFile(t *testing.T) {
	tmpDir := t.TempDir()

	pieces := [][]byte{
		[]byte("AAAAAAAAAAAA"),
		[]byte("BBBBBBBBBBBB"),
		[]byte("CCCCCCCCCCCC"),
		[]byte("DDDD"),
	}

	var hashes [][20]byte
	var length int64
	for _, p := range pieces {
		hashes = append(hashes, sha1.Sum(p))
		length += int64(len(p))
	}

	tor := &torrent.Torrent{
		Name:        "multi-resume-test",
		Length:      length,
		PieceLength: len(pieces[0]),
		Files: []torrent.FileInfo{
			{Path: []string{"fileA.bin"}, Length: 15, Offset: 0},
			{Path: []string{"fileB.bin"}, Length: 15, Offset: 15},
			{Path: []string{"fileC.bin"}, Length: 10, Offset: 30},
		},
		Pieces:   hashes,
		InfoHash: sha1.Sum([]byte("fake-info-hash-for-multifile-e2e-test")),
	}

	peerID, err := peer.New()
	if err != nil {
		t.Fatalf("peer.New: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	fp1 := runInterruptedRound(t, tor, peerID, tmpDir, pieces, logger, 2)
	defer fp1.stop()

	completeAfterRound1 := verifyPiecesOnDiskMultiFile(t, tmpDir, tor, pieces)
	if len(completeAfterRound1) == 0 || len(completeAfterRound1) == len(pieces) {
		t.Fatalf("expected partial completion after round 1, got %v", completeAfterRound1)
	}

	fp2 := startFakePeer(t, tor.InfoHash, allPieces(len(pieces)), alwaysServe(pieces), 0)
	defer fp2.stop()

	d2 := &Downloader{
		Torrent:    tor,
		PeerID:     peerID,
		Port:       6881,
		OutputDir:  tmpDir,
		Peers:      []tracker.Peer{mustParsePeerAddr(t, fp2.addr())},
		logger:     logger.With("component", "downloader"),
		baseLogger: logger,
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	if err := d2.Download(ctx2); err != nil {
		t.Fatalf("round 2 (resume) failed: %v", err)
	}

	requestedRound2 := fp2.requestedIndices()
	for _, idx := range completeAfterRound1 {
		for _, r := range requestedRound2 {
			if r == idx {
				t.Fatalf("piece %d was already correct on disk after round 1 but was re-requested from the network in round 2", idx)
			}
		}
	}

	finalComplete := verifyPiecesOnDiskMultiFile(t, tmpDir, tor, pieces)
	if len(finalComplete) != len(pieces) {
		t.Fatalf("expected all %d pieces present after round 2, got %d: %v", len(pieces), len(finalComplete), finalComplete)
	}

	wantFileA := "AAAAAAAAAAAABBB"
	wantFileB := "BBBBBBBBBCCCCCC"
	wantFileC := "CCCCCCDDDD"

	gotA, err := os.ReadFile(filepath.Join(tmpDir, tor.Name, "fileA.bin"))
	if err != nil {
		t.Fatalf("read fileA.bin: %v", err)
	}
	gotB, err := os.ReadFile(filepath.Join(tmpDir, tor.Name, "fileB.bin"))
	if err != nil {
		t.Fatalf("read fileB.bin: %v", err)
	}
	gotC, err := os.ReadFile(filepath.Join(tmpDir, tor.Name, "fileC.bin"))
	if err != nil {
		t.Fatalf("read fileC.bin: %v", err)
	}

	if string(gotA) != wantFileA {
		t.Fatalf("fileA.bin content mismatch: got %q, want %q", gotA, wantFileA)
	}
	if string(gotB) != wantFileB {
		t.Fatalf("fileB.bin content mismatch: got %q, want %q", gotB, wantFileB)
	}
	if string(gotC) != wantFileC {
		t.Fatalf("fileC.bin content mismatch: got %q, want %q", gotC, wantFileC)
	}
}

func TestResumeRateTracking(t *testing.T) {
	tmpDir := t.TempDir()

	const numPieces = 10
	const pieceSize = 100
	const perPieceDelay = 20 * time.Millisecond

	var pieces [][]byte
	for i := 0; i < numPieces; i++ {
		p := make([]byte, pieceSize)
		for j := range p {
			p[j] = byte('A' + i)
		}
		pieces = append(pieces, p)
	}

	var hashes [][20]byte
	var length int64
	for _, p := range pieces {
		hashes = append(hashes, sha1.Sum(p))
		length += int64(len(p))
	}

	tor := &torrent.Torrent{
		Name:        "rate-test.bin",
		Length:      length,
		PieceLength: pieceSize,
		Pieces:      hashes,
		InfoHash:    sha1.Sum([]byte("fake-info-hash-for-rate-test")),
	}

	peerID, err := peer.New()
	if err != nil {
		t.Fatalf("peer.New: %v", err)
	}

	quietLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	fp1 := runInterruptedRound(t, tor, peerID, tmpDir, pieces, quietLogger, 3)
	defer fp1.stop()

	outputPath := filepath.Join(tmpDir, tor.Name)
	completeAfterRound1 := verifyPiecesOnDiskSingleFile(t, outputPath, pieces)
	if len(completeAfterRound1) == 0 || len(completeAfterRound1) == numPieces {
		t.Fatalf("expected partial completion after round 1, got %v", completeAfterRound1)
	}

	var logBuf bytes.Buffer
	jsonLogger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	fp2 := startFakePeer(t, tor.InfoHash, allPieces(numPieces), alwaysServe(pieces), perPieceDelay)
	defer fp2.stop()

	d2 := &Downloader{
		Torrent:    tor,
		PeerID:     peerID,
		Port:       6881,
		OutputDir:  tmpDir,
		Peers:      []tracker.Peer{mustParsePeerAddr(t, fp2.addr())},
		logger:     jsonLogger.With("component", "downloader"),
		baseLogger: jsonLogger,
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()

	if err := d2.Download(ctx2); err != nil {
		t.Fatalf("round 2 (resume) failed: %v", err)
	}

	var progressEntries []map[string]any
	for _, line := range bytes.Split(logBuf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("failed to parse log line as JSON: %v (line: %s)", err, line)
		}
		if entry["msg"] == "download progress" {
			progressEntries = append(progressEntries, entry)
		}
	}

	if len(progressEntries) == 0 {
		t.Fatal("expected at least one 'download progress' log entry in round 2")
	}

	firstRate, ok := progressEntries[0]["bytes_per_sec"].(float64)
	if !ok {
		t.Fatalf("bytes_per_sec missing or wrong type in first progress entry: %v", progressEntries[0])
	}

	const maxPlausibleRate = pieceSize / 0.005
	if firstRate > maxPlausibleRate {
		t.Errorf("first progress rate (%v B/s) looks like it double-counted resumed bytes as instantly downloaded (max plausible ~%v B/s)", firstRate, float64(maxPlausibleRate))
	}

	for i, entry := range progressEntries {
		rate, _ := entry["bytes_per_sec"].(float64)
		if rate < 0 {
			t.Errorf("entry %d: bytes_per_sec should never be negative, got %v", i, rate)
		}
	}

	finalComplete := verifyPiecesOnDiskSingleFile(t, outputPath, pieces)
	if len(finalComplete) != numPieces {
		t.Fatalf("expected all %d pieces complete after round 2, got %d: %v", numPieces, len(finalComplete), finalComplete)
	}
}
