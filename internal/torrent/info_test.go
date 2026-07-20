package torrent

import (
	"strings"
	"testing"
)

func multiFileInfo(name string, pieceLength int64, files []struct {
	path   []string
	length int64
}) string {
	var filesList strings.Builder
	var total int64
	for _, f := range files {
		var pathList strings.Builder
		for _, p := range f.path {
			pathList.WriteString(bStr(p))
		}
		filesList.WriteString("d" +
			bStr("length") + bInt(f.length) +
			bStr("path") + "l" + pathList.String() + "e" +
			"e")
		total += f.length
	}

	return bStr("files") + "l" + filesList.String() + "e" +
		bStr("name") + bStr(name) +
		bStr("piece length") + bInt(pieceLength) +
		bStr("pieces") + piecesField(expectedPieceCount(total, pieceLength))
}

func TestParseInfoNameValidation(t *testing.T) {
	cases := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: "normal-file.iso", wantErr: false},
		{name: "../../../etc/passwd", wantErr: true},
		{name: "..", wantErr: true},
		{name: ".", wantErr: true},
		{name: "", wantErr: true},
		{name: "/etc/passwd", wantErr: true},
		{name: "a/b", wantErr: true},
		{name: "a\\b", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := buildTorrent("", singleFileInfo(c.name, 100, 100))
			_, err := Parse(data)
			if c.wantErr && err == nil {
				t.Errorf("name=%q: expected error, got none", c.name)
			}
			if !c.wantErr && err != nil {
				t.Errorf("name=%q: expected no error, got %v", c.name, err)
			}
		})
	}
}

func TestParseInfoFilePathValidation(t *testing.T) {
	cases := []struct {
		name    string
		path    []string
		wantErr bool
	}{
		{name: "normal nested path", path: []string{"subdir", "file.txt"}, wantErr: false},
		{name: "path traversal", path: []string{"../../../etc/passwd"}, wantErr: true},
		{name: "dotdot component", path: []string{".."}, wantErr: true},
		{name: "dot component", path: []string{"subdir", "."}, wantErr: true},
		{name: "empty component", path: []string{""}, wantErr: true},
		{name: "embedded slash", path: []string{"a/b"}, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := multiFileInfo("root", 100, []struct {
				path   []string
				length int64
			}{
				{path: c.path, length: 100},
			})
			data := buildTorrent("", info)
			_, err := Parse(data)
			if c.wantErr && err == nil {
				t.Errorf("path=%v: expected error, got none", c.path)
			}
			if !c.wantErr && err != nil {
				t.Errorf("path=%v: expected no error, got %v", c.path, err)
			}
		})
	}
}

func TestParseInfoPieceLengthCap(t *testing.T) {
	cases := []struct {
		name        string
		pieceLength int64
		wantErr     bool
	}{
		{"typical 256KiB", 256 * 1024, false},
		{"at cap 256MiB", 256 * 1024 * 1024, false},
		{"over cap", 256*1024*1024 + 1, true},
		{"zero", 0, true},
		{"negative", -1, true},
		{"absurdly large", 1 << 62, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pieceLength := c.pieceLength
			if pieceLength <= 0 {
				pieceLength = 1
			}
			length := pieceLength
			info := bStr("length") + bInt(length) +
				bStr("name") + bStr("f.bin") +
				bStr("piece length") + bInt(c.pieceLength) +
				bStr("pieces") + piecesField(1)
			data := buildTorrent("", info)

			_, err := Parse(data)
			if c.wantErr && err == nil {
				t.Errorf("pieceLength=%d: expected error, got none", c.pieceLength)
			}
			if !c.wantErr && err != nil {
				t.Errorf("pieceLength=%d: expected no error, got %v", c.pieceLength, err)
			}
		})
	}
}

func TestParseInfoDuplicateFilePaths(t *testing.T) {
	info := multiFileInfo("root", 100, []struct {
		path   []string
		length int64
	}{
		{path: []string{"a.txt"}, length: 100},
		{path: []string{"a.txt"}, length: 100},
	})
	data := buildTorrent("", info)

	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected duplicate file path to be rejected")
	}
}

func TestParseInfoDistinctFilePathsAllowed(t *testing.T) {
	info := multiFileInfo("root", 100, []struct {
		path   []string
		length int64
	}{
		{path: []string{"a.txt"}, length: 100},
		{path: []string{"b.txt"}, length: 100},
	})
	data := buildTorrent("", info)

	tor, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tor.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(tor.Files))
	}
}

func TestParseInfoFileCountCap(t *testing.T) {
	var files []struct {
		path   []string
		length int64
	}
	for i := 0; i < 10001; i++ {
		files = append(files, struct {
			path   []string
			length int64
		}{path: []string{"f", string(rune('a' + i%26))}, length: 1})
	}

	info := multiFileInfo("root", 100, files)
	data := buildTorrent("", info)

	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected file count over the cap to be rejected")
	}
}

func TestParseInfoPieceCountCrossCheck(t *testing.T) {
	cases := []struct {
		name        string
		length      int64
		pieceLength int64
		pieceCount  int
		wantErr     bool
	}{
		{"correct count", 250, 100, 3, false},
		{"too few pieces", 250, 100, 2, true},
		{"too many pieces", 250, 100, 4, true},
		{"exact multiple correct", 200, 100, 2, false},
		{"exact multiple wrong", 200, 100, 3, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := bStr("length") + bInt(c.length) +
				bStr("name") + bStr("f.bin") +
				bStr("piece length") + bInt(c.pieceLength) +
				bStr("pieces") + piecesField(c.pieceCount)
			data := buildTorrent("", info)

			_, err := Parse(data)
			if c.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !c.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestParseInfoFileLengthOverflowGuard(t *testing.T) {
	fileEntry := func(name string, length int64) string {
		return "d" +
			bStr("length") + bInt(length) +
			bStr("path") + "l" + bStr(name) + "e" +
			"e"
	}

	filesList := fileEntry("a.bin", 1<<62) + fileEntry("b.bin", 1<<62)

	info := bStr("files") + "l" + filesList + "e" +
		bStr("name") + bStr("root") +
		bStr("piece length") + bInt(100) +
		bStr("pieces") + piecesField(1)

	data := buildTorrent("", info)

	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected cumulative length overflow to be rejected")
	}
	if !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("expected an overflow-specific error, got: %v", err)
	}
}
