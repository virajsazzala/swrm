package torrent

import (
	"errors"
	"fmt"

	"github.com/virajsazzala/swrm/internal/bencode"
)

func parseInfo(t *Torrent, root map[string]any) error {
	/*
		todo:
			support multi file torrents ("files" field)
	*/

	// get info map
	info, err := getDict(root, "info", true)
	if err != nil {
		return err
	}

	// get name from info map
	name, err := getString(info, "name", true)
	if err != nil {
		return err
	}
	t.Name = name

	// get length from info map
	length, err := getInt(info, "length", true)
	if err != nil {
		return err
	}
	if length < 0 {
		return fmt.Errorf("Invalid length: %v", length)
	}
	t.Length = length

	// get piece length from info map
	pieceLength, err := getInt(info, "piece length", true)
	if err != nil {
		return err
	}
	if pieceLength <= 0 {
		return fmt.Errorf("Invalid pieces length: %v", pieceLength)
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
