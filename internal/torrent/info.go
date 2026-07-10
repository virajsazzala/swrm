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
	i, err := getDict(root, "info", true)
	if err != nil {
		return err
	}

	// get name from info map
	s, err := getString(i, "name", true)
	if err != nil {
		return err
	}
	t.Name = s

	// get length from info map
	n, err := getInt(i, "length", true)
	if err != nil {
		return err
	}
	if n < 0 {
		return fmt.Errorf("Invalid length: %v", n)
	}
	t.Length = n

	// get piece length from info map
	n, err = getInt(i, "piece length", true)
	if err != nil {
		return err
	}
	if n <= 0 {
		return fmt.Errorf("Invalid pieces length: %v", n)
	}
	t.PieceLength = int(n)

	ps, err := getString(i, "pieces", true)
	if err != nil {
		return err
	}
	t.Pieces, err = splitPieces(ps)
	if err != nil {
		return err
	}

	return nil
}

func splitPieces(s string) ([][20]byte, error) {
	pb := []byte(s)
	pl := len(pb)
	if pl%20 != 0 || pl == 0 {
		return nil, fmt.Errorf("Invalid byte count in pieces field: %v", pl)
	}

	pr := make([][20]byte, 0, pl/20)

	for i := 0; i < pl; i += 20 {
		var h [20]byte
		copy(h[:], pb[i:i+20])
		pr = append(pr, h)
	}

	return pr, nil
}

func findInfoBytes(b []byte) ([]byte, error) {
	if len(b) == 0 || b[0] != 'd' {
		return nil, errors.New("torrent root must be a dictionary")
	}

	i := 1
	for {
		if i >= len(b) {
			return nil, errors.New("unterminated dictionary")
		}

		if b[i] == 'e' {
			break
		}

		k, n, err := bencode.ReadString(b[i:])
		if err != nil {
			return nil, err
		}
		i += n

		if k == "info" {
			s, err := bencode.ValueSize(b[i:])
			if err != nil {
				return nil, err
			}
			return b[i : i+s], nil
		}

		s, err := bencode.ValueSize(b[i:])
		if err != nil {
			return nil, err
		}

		i += s
	}

	return nil, fmt.Errorf("Info field not found")
}
