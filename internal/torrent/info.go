package torrent

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/virajsazzala/swrm/internal/bencode"
)

const maxPieceLength = 256 * 1024 * 1024

func parseInfo(t *Torrent, root map[string]any) error {
	// get info map
	info, err := getDict(root, "info", true)
	if err != nil {
		return err
	}

	name, err := getString(info, "name", true)
	if err != nil {
		return err
	}
	if !isSafePathComponent(name) {
		return fmt.Errorf("invalid torrent name %q", name)
	}
	t.Name = name

	if filesRaw, ok := info["files"]; ok {
		if err := parseFiles(t, filesRaw); err != nil {
			return err
		}
	} else {
		length, err := getInt(info, "length", true)
		if err != nil {
			return err
		}
		if length < 0 {
			return fmt.Errorf("Invalid length: %v", length)
		}
		t.Length = length
	}

	// get piece length from info map
	pieceLength, err := getInt(info, "piece length", true)
	if err != nil {
		return err
	}
	if pieceLength <= 0 {
		return fmt.Errorf("Invalid pieces length: %v", pieceLength)
	}
	if pieceLength > maxPieceLength {
		return fmt.Errorf("piece length %d exceeds maximum allowed %d", pieceLength, maxPieceLength)
	}
	t.PieceLength = int(pieceLength)

	pieces, err := getString(info, "pieces", true)
	if err != nil {
		return err
	}
	t.Pieces, err = splitPieces(pieces)
	if err != nil {
		return err
	}

	return nil
}

func parseFiles(t *Torrent, filesRaw any) error {
	files, ok := filesRaw.([]any)
	if !ok {
		return fmt.Errorf("files field must be a list")
	}
	if len(files) == 0 {
		return fmt.Errorf("files field must not be empty")
	}

	var offset int64
	for i, fv := range files {
		fd, ok := fv.(map[string]any)
		if !ok {
			return fmt.Errorf("file entry %d must be a dictionary", i)
		}

		length, err := getInt(fd, "length", true)
		if err != nil {
			return fmt.Errorf("file entry %d: %w", i, err)
		}
		if length < 0 {
			return fmt.Errorf("file entry %d: invalid length: %v", i, length)
		}
		if offset > math.MaxInt64-length {
			return fmt.Errorf("file entry %d: cumulative length overflows", i)
		}

		path, err := parseFilePath(fd, i)
		if err != nil {
			return err
		}

		t.Files = append(t.Files, FileInfo{
			Path:   path,
			Length: length,
			Offset: offset,
		})

		offset += length
	}

	t.Length = offset

	return nil
}

func parseFilePath(fd map[string]any, index int) ([]string, error) {
	pathRaw, ok := fd["path"]
	if !ok {
		return nil, fmt.Errorf("file entry %d: missing path", index)
	}

	pathList, ok := pathRaw.([]any)
	if !ok || len(pathList) == 0 {
		return nil, fmt.Errorf("file entry %d: invalid path", index)
	}

	path := make([]string, 0, len(pathList))
	for _, pv := range pathList {
		component, ok := pv.(string)
		if !ok {
			return nil, fmt.Errorf("file entry %d: path component must be a string", index)
		}

		if !isSafePathComponent(component) {
			return nil, fmt.Errorf("file entry %d: invalid path component %q", index, component)
		}

		path = append(path, component)
	}

	return path, nil
}

func isSafePathComponent(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	if strings.ContainsAny(s, "/\\") {
		return false
	}
	if strings.ContainsRune(s, 0) {
		return false
	}
	return true
}

func splitPieces(s string) ([][20]byte, error) {
	data := []byte(s)
	count := len(data)
	if count%20 != 0 || count == 0 {
		return nil, fmt.Errorf("Invalid byte count in pieces field: %v", count)
	}

	pieces := make([][20]byte, 0, count/20)

	for i := 0; i < count; i += 20 {
		var hash [20]byte
		copy(hash[:], data[i:i+20])
		pieces = append(pieces, hash)
	}

	return pieces, nil
}

func findInfoBytes(b []byte) ([]byte, error) {
	if len(b) == 0 || b[0] != 'd' {
		return nil, errors.New("torrent root must be a dictionary")
	}

	offset := 1
	for {
		if offset >= len(b) {
			return nil, errors.New("unterminated dictionary")
		}

		if b[offset] == 'e' {
			break
		}

		key, consumed, err := bencode.ReadString(b[offset:])
		if err != nil {
			return nil, err
		}
		offset += consumed

		if key == "info" {
			size, err := bencode.ValueSize(b[offset:])
			if err != nil {
				return nil, err
			}
			return b[offset : offset+size], nil
		}

		size, err := bencode.ValueSize(b[offset:])
		if err != nil {
			return nil, err
		}

		offset += size
	}

	return nil, fmt.Errorf("Info field not found")
}
