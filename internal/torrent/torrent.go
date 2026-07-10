package torrent

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/virajsazzala/swrm/internal/bencode"
)

type Torrent struct {
	Announce    string
	Name        string
	Length      int64
	PieceLength int
	Pieces      [][20]byte
	InfoHash    [20]byte

	Comment      string
	CreatedBy    string
	CreationDate time.Time
}

func Open(p string) (*Torrent, error) {
	d, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read torrent: %w", err)
	}

	t, err := Parse(d)
	if err != nil {
		return nil, fmt.Errorf("parse torrent: %w", err)
	}

	return t, nil
}

func Parse(b []byte) (*Torrent, error) {
	v, err := bencode.Unmarshal(b)
	if err != nil {
		return nil, err
	}

	r, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("torrent root must be a dictionary")
	}

	t := &Torrent{}

	// get announce
	s, err := getString(r, "announce", true)
	if err != nil {
		return nil, err
	}
	t.Announce = s

	// get created by
	s, err = getString(r, "created by", false)
	if err != nil {
		return nil, err
	}
	t.CreatedBy = s

	// get creation date
	ts, err := getInt(r, "creation date", false)
	if err != nil {
		return nil, err
	}
	if ts != 0 {
		t.CreationDate = time.Unix(ts, 0)
	}

	// get comment
	s, err = getString(r, "comment", false)
	if err != nil {
		return nil, err
	}
	t.Comment = s

	err = parseInfo(t, r)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func getString(root map[string]any, key string, req bool) (string, error) {
	v, ok := root[key]
	if !ok {
		if req {
			return "", fmt.Errorf("missing required field: %s", key)
		}
		return "", nil
	}

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %s must be a string", key)
	}

	return s, nil
}

func getInt(root map[string]any, key string, req bool) (int64, error) {
	v, ok := root[key]
	if !ok {
		if req {
			return 0, fmt.Errorf("missing required field: %s", key)
		}
		return 0, nil
	}

	i, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("field %s must be an integer", key)
	}

	return i, nil
}

func getDict(root map[string]any, key string, req bool) (map[string]any, error) {
	v, ok := root[key]
	if !ok {
		if req {
			return nil, fmt.Errorf("missing required field: %s", key)
		}
		return nil, nil
	}

	d, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("field %s must be a dictionary", key)
	}

	return d, nil

}

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
