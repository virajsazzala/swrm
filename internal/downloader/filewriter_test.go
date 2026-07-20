package downloader

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/virajsazzala/swrm/internal/torrent"
)

func TestFileWriterSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	tor := &torrent.Torrent{Name: "single.bin", Length: 20}

	fw, err := newFileWriter(tor, tmpDir)
	if err != nil {
		t.Fatalf("newFileWriter: %v", err)
	}
	defer fw.Close()

	data := bytes.Repeat([]byte{0xAA}, 20)
	if err := fw.WriteAt(data, 0); err != nil {
		t.Fatalf("WriteAt: %v", err)
	}

	got := make([]byte, 20)
	if err := fw.ReadAt(got, 0); err != nil {
		t.Fatalf("ReadAt: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %v, want %v", got, data)
	}

	onDisk, err := os.ReadFile(filepath.Join(tmpDir, "single.bin"))
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(onDisk, data) {
		t.Fatalf("on-disk content = %v, want %v", onDisk, data)
	}
}

func TestFileWriterMultiFileSpanningBoundaries(t *testing.T) {
	tmpDir := t.TempDir()
	tor := &torrent.Torrent{
		Name:   "multi-root",
		Length: 40,
		Files: []torrent.FileInfo{
			{Path: []string{"fileA.bin"}, Length: 15, Offset: 0},
			{Path: []string{"fileB.bin"}, Length: 15, Offset: 15},
			{Path: []string{"fileC.bin"}, Length: 10, Offset: 30},
		},
	}

	fw, err := newFileWriter(tor, tmpDir)
	if err != nil {
		t.Fatalf("newFileWriter: %v", err)
	}
	defer fw.Close()

	if err := fw.WriteAt(bytes.Repeat([]byte{'A'}, 12), 0); err != nil {
		t.Fatalf("write piece 0: %v", err)
	}
	if err := fw.WriteAt(bytes.Repeat([]byte{'B'}, 12), 12); err != nil {
		t.Fatalf("write piece 1: %v", err)
	}
	if err := fw.WriteAt(bytes.Repeat([]byte{'C'}, 12), 24); err != nil {
		t.Fatalf("write piece 2: %v", err)
	}
	if err := fw.WriteAt(bytes.Repeat([]byte{'D'}, 4), 36); err != nil {
		t.Fatalf("write piece 3: %v", err)
	}

	wantA := "AAAAAAAAAAAABBB"
	wantB := "BBBBBBBBBCCCCCC"
	wantC := "CCCCCCDDDD"

	gotA, err := os.ReadFile(filepath.Join(tmpDir, "multi-root", "fileA.bin"))
	if err != nil {
		t.Fatalf("read fileA.bin: %v", err)
	}
	gotB, err := os.ReadFile(filepath.Join(tmpDir, "multi-root", "fileB.bin"))
	if err != nil {
		t.Fatalf("read fileB.bin: %v", err)
	}
	gotC, err := os.ReadFile(filepath.Join(tmpDir, "multi-root", "fileC.bin"))
	if err != nil {
		t.Fatalf("read fileC.bin: %v", err)
	}

	if string(gotA) != wantA {
		t.Errorf("fileA.bin = %q, want %q", gotA, wantA)
	}
	if string(gotB) != wantB {
		t.Errorf("fileB.bin = %q, want %q", gotB, wantB)
	}
	if string(gotC) != wantC {
		t.Errorf("fileC.bin = %q, want %q", gotC, wantC)
	}

	got := make([]byte, 40)
	if err := fw.ReadAt(got, 0); err != nil {
		t.Fatalf("ReadAt across all files: %v", err)
	}
	fullWant := "AAAAAAAAAAAA" + "BBBBBBBBBBBB" + "CCCCCCCCCCCC" + "DDDD"
	if string(got) != fullWant {
		t.Fatalf("ReadAt(0, 40) = %q, want %q", got, fullWant)
	}
}

func TestFileWriterOffsetOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	tor := &torrent.Torrent{Name: "f.bin", Length: 10}

	fw, err := newFileWriter(tor, tmpDir)
	if err != nil {
		t.Fatalf("newFileWriter: %v", err)
	}
	defer fw.Close()

	if err := fw.WriteAt([]byte{1}, 10); err == nil {
		t.Fatal("expected error writing at an out-of-range offset")
	}
	if err := fw.ReadAt(make([]byte, 1), 10); err == nil {
		t.Fatal("expected error reading at an out-of-range offset")
	}
}

func TestFileWriterCreatesOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "does", "not", "exist", "yet")
	tor := &torrent.Torrent{Name: "f.bin", Length: 5}

	fw, err := newFileWriter(tor, nested)
	if err != nil {
		t.Fatalf("newFileWriter: %v", err)
	}
	defer fw.Close()

	if _, err := os.Stat(filepath.Join(nested, "f.bin")); err != nil {
		t.Fatalf("expected output file to exist under the auto-created directory: %v", err)
	}
}

func TestFileWriterSync(t *testing.T) {
	tmpDir := t.TempDir()
	tor := &torrent.Torrent{Name: "f.bin", Length: 5}

	fw, err := newFileWriter(tor, tmpDir)
	if err != nil {
		t.Fatalf("newFileWriter: %v", err)
	}
	defer fw.Close()

	if err := fw.WriteAt([]byte{1, 2, 3, 4, 5}, 0); err != nil {
		t.Fatalf("WriteAt: %v", err)
	}
	if err := fw.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
}
