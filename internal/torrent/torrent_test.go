package torrent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bStr(s string) string { return fmt.Sprintf("%d:%s", len(s), s) }
func bInt(i int64) string  { return fmt.Sprintf("i%de", i) }

func expectedPieceCount(length, pieceLength int64) int {
	n := length / pieceLength
	if length%pieceLength != 0 {
		n++
	}
	if n == 0 {
		n = 1
	}
	return int(n)
}

func piecesField(count int) string {
	return bStr(strings.Repeat("\x00", 20*count))
}

func buildTorrent(topLevelExtra, infoContent string) []byte {
	info := "d" + infoContent + "e"
	top := "d" + bStr("announce") + bStr("udp://example.com") +
		topLevelExtra +
		bStr("info") + info + "e"
	return []byte(top)
}

func singleFileInfo(name string, pieceLength, length int64) string {
	pieceLength = max(pieceLength, 1)
	return bStr("length") + bInt(length) +
		bStr("name") + bStr(name) +
		bStr("piece length") + bInt(pieceLength) +
		bStr("pieces") + piecesField(expectedPieceCount(length, pieceLength))
}

func TestParseSingleFile(t *testing.T) {
	data := buildTorrent("", singleFileInfo("sample.txt", 100, 250))

	tor, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tor.Name != "sample.txt" {
		t.Errorf("Name = %q, want %q", tor.Name, "sample.txt")
	}
	if tor.Length != 250 {
		t.Errorf("Length = %d, want 250", tor.Length)
	}
	if tor.PieceLength != 100 {
		t.Errorf("PieceLength = %d, want 100", tor.PieceLength)
	}
	if len(tor.Pieces) != 3 {
		t.Errorf("len(Pieces) = %d, want 3", len(tor.Pieces))
	}
	if len(tor.Files) != 0 {
		t.Errorf("expected no Files for single-file torrent, got %d", len(tor.Files))
	}
	if tor.Announce != "udp://example.com" {
		t.Errorf("Announce = %q, want %q", tor.Announce, "udp://example.com")
	}
}

func TestParseAnnounceOptionalWithAnnounceList(t *testing.T) {
	announceList := bStr("announce-list") + "l" +
		"l" + bStr("udp://tracker-a.example") + bStr("udp://tracker-b.example") + "e" +
		"e"

	info := "d" + singleFileInfo("f.bin", 100, 100) + "e"
	top := "d" + announceList + bStr("info") + info + "e"

	tor, err := Parse([]byte(top))
	if err != nil {
		t.Fatalf("expected announce to be optional when announce-list is present, got: %v", err)
	}

	if tor.Announce != "" {
		t.Errorf("Announce = %q, want empty", tor.Announce)
	}
	if len(tor.AnnounceList) != 1 || len(tor.AnnounceList[0]) != 2 {
		t.Fatalf("unexpected AnnounceList: %v", tor.AnnounceList)
	}
}

func TestTrackersOrderingAndDedup(t *testing.T) {
	tor := &Torrent{
		Announce: "udp://a.example",
		AnnounceList: [][]string{
			{"udp://b.example", "udp://a.example"},
			{"udp://c.example"},
		},
	}

	got := tor.Trackers()
	want := []string{"udp://b.example", "udp://a.example", "udp://c.example"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestTrackersEmpty(t *testing.T) {
	tor := &Torrent{}
	if got := tor.Trackers(); len(got) != 0 {
		t.Errorf("expected no trackers, got %v", got)
	}
}

func TestPieceByteLength(t *testing.T) {
	tor := &Torrent{
		Length:      250,
		PieceLength: 100,
		Pieces:      make([][20]byte, 3),
	}

	cases := []struct {
		index int
		want  int
	}{
		{0, 100},
		{1, 100},
		{2, 50},
	}
	for _, c := range cases {
		if got := tor.PieceByteLength(c.index); got != c.want {
			t.Errorf("PieceByteLength(%d) = %d, want %d", c.index, got, c.want)
		}
	}
}

func TestPieceByteLengthExactMultiple(t *testing.T) {
	tor := &Torrent{
		Length:      200,
		PieceLength: 100,
		Pieces:      make([][20]byte, 2),
	}

	if got := tor.PieceByteLength(1); got != 100 {
		t.Errorf("PieceByteLength(last) = %d, want 100 (exact multiple, no short piece)", got)
	}
}

func TestFileListSingleFile(t *testing.T) {
	tor := &Torrent{Name: "movie.mp4", Length: 500}
	list := tor.FileList()
	if len(list) != 1 || list[0].Path[0] != "movie.mp4" || list[0].Length != 500 {
		t.Fatalf("unexpected FileList: %+v", list)
	}
}

func TestRealTorrentFilesParseCleanly(t *testing.T) {
	dir := "../../assets/torrent-files"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	found := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".torrent" {
			continue
		}
		found++

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("%s: read: %v", e.Name(), err)
		}

		tor, err := Parse(data)
		if err != nil {
			t.Errorf("%s: expected to parse cleanly, got error: %v", e.Name(), err)
			continue
		}
		if tor.PieceLength <= 0 {
			t.Errorf("%s: invalid PieceLength %d", e.Name(), tor.PieceLength)
		}
		if len(tor.Pieces) == 0 {
			t.Errorf("%s: expected at least one piece", e.Name())
		}
	}

	if found == 0 {
		t.Fatal("no .torrent files found under assets/torrent-files")
	}
}
