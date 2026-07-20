package torrent

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/virajsazzala/swrm/internal/bencode"
)

type FileInfo struct {
	Path   []string
	Length int64
	Offset int64
}

type Torrent struct {
	Announce     string
	AnnounceList [][]string
	Name         string
	Length       int64
	PieceLength  int
	Pieces       [][20]byte
	InfoHash     [20]byte
	Files        []FileInfo
	Comment      string
	CreatedBy    string
	CreationDate time.Time
}

func (t *Torrent) FileList() []FileInfo {
	if len(t.Files) > 0 {
		return t.Files
	}
	return []FileInfo{{Path: []string{t.Name}, Length: t.Length, Offset: 0}}
}

func (t *Torrent) Trackers() []string {
	var ordered []string
	seen := make(map[string]bool)

	add := func(u string) {
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		ordered = append(ordered, u)
	}

	for _, tier := range t.AnnounceList {
		for _, u := range tier {
			add(u)
		}
	}

	add(t.Announce)

	return ordered
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

	// get announce (optional per BEP 12 when announce-list is present)
	str, err := getString(dict, "announce", false)
	if err != nil {
		return nil, err
	}
	t.Announce = str

	// get announce-list (optional, BEP 12)
	if v, ok := dict["announce-list"]; ok {
		list, err := parseAnnounceList(v)
		if err != nil {
			return nil, err
		}
		t.AnnounceList = list
	}

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

func parseAnnounceList(v any) ([][]string, error) {
	tiers, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("announce-list must be a list")
	}

	result := make([][]string, 0, len(tiers))
	for i, tierRaw := range tiers {
		tierList, ok := tierRaw.([]any)
		if !ok {
			return nil, fmt.Errorf("announce-list tier %d must be a list", i)
		}

		tier := make([]string, 0, len(tierList))
		for _, urlRaw := range tierList {
			urlStr, ok := urlRaw.(string)
			if !ok {
				return nil, fmt.Errorf("announce-list tier %d: url must be a string", i)
			}
			tier = append(tier, urlStr)
		}

		result = append(result, tier)
	}

	return result, nil
}
