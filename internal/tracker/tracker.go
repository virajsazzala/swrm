package tracker

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/virajsazzala/swrm/internal/torrent"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

func Announce(tor *torrent.Torrent, peerID [20]byte, port uint16) (*Response, error) {
	/*
		note:
			better handling for failure, like tracker overload, etc.
	*/
	// build url
	u, err := url.Parse(tor.Announce)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("info_hash", string(tor.InfoHash[:]))
	q.Set("peer_id", string(peerID[:]))
	q.Set("port", strconv.Itoa(int(port)))
	q.Set("uploaded", "0")
	q.Set("downloaded", "0")
	q.Set("left", strconv.FormatInt(tor.Length, 10))
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
