package tracker

import (
	"errors"

	"github.com/virajsazzala/swrm/internal/bencode"
)

type Response struct {
	Interval int64
	Peers    []Peer
}

func parseResponse(body []byte) (*Response, error) {
	v, err := bencode.Unmarshal(body)
	if err != nil {
		return nil, err
	}
	r, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("tracker response root must be a dictionary")
	}

	// parse peers
	pst, ok := r["peers"].(string)
	if !ok {
		return nil, errors.New("peers field is missing or not a string")
	}

	prs, err := parseCompactPeers(pst)
	if err != nil {
		return nil, err
	}

	// parse interval
	ivl, ok := r["interval"].(int64)
	if !ok {
		return nil, errors.New("interval field is missing or not an integer")
	}

	return &Response{Interval: ivl, Peers: prs}, nil
}
