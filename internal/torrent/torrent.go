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
