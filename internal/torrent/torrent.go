package torrent

import (
	"crypto/sha1"
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

	ib, err := findInfoBytes(b)
	if err != nil {
		return nil, err
	}
	t.InfoHash = sha1.Sum(ib)

	return t, nil
}
