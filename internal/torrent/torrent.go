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

func Open(path string) (*Torrent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read torrent: %w", err)
	}

	t, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse torrent: %w", err)
	}

	return t, nil
}

func Parse(b []byte) (*Torrent, error) {
	value, err := bencode.Unmarshal(b)
	if err != nil {
		return nil, err
	}

	dict, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("torrent root must be a dictionary")
	}

	t := &Torrent{}

	// get announce
	str, err := getString(dict, "announce", true)
	if err != nil {
		return nil, err
	}
	t.Announce = str

	// get created by
	str, err = getString(dict, "created by", false)
	if err != nil {
		return nil, err
	}
	t.CreatedBy = str

	// get creation date
	timestamp, err := getInt(dict, "creation date", false)
	if err != nil {
		return nil, err
	}
	if timestamp != 0 {
		t.CreationDate = time.Unix(timestamp, 0)
	}

	// get comment
	str, err = getString(dict, "comment", false)
	if err != nil {
		return nil, err
	}
	t.Comment = str

	err = parseInfo(t, dict)
	if err != nil {
		return nil, err
	}

	infobytes, err := findInfoBytes(b)
	if err != nil {
		return nil, err
	}
	t.InfoHash = sha1.Sum(infobytes)

	return t, nil
}
