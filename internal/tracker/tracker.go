package tracker

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/virajsazzala/swrm/internal/torrent"
)

func Announce(t *torrent.Torrent, pid [20]byte, pt uint16) (*Response, error) {
	/*
		note:
			better handling for failure, like tracker overload, etc.
	*/
	// build url
	u, err := url.Parse(t.Announce)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("info_hash", string(t.InfoHash[:]))
	q.Set("peer_id", string(pid[:]))
	q.Set("port", strconv.Itoa(int(pt)))
	q.Set("uploaded", "0")
	q.Set("downloaded", "0")
	q.Set("left", strconv.FormatInt(t.Length, 10))
	q.Set("compact", "1")

	u.RawQuery = q.Encode()

	// request tracker
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tracker request status expected 200, got %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseResponse(body)
}
