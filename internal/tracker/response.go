package tracker

import (
	"errors"
	"fmt"

	"github.com/virajsazzala/swrm/internal/bencode"
)

type Response struct {
	Interval int64
	Peers    []Peer
}

func parseResponse(body []byte) (*Response, error) {
	value, err := bencode.Unmarshal(body)
	if err != nil {
		return nil, err
	}

	dict, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("tracker response root must be a dictionary")
	}

	if reason, ok := dict["failure reason"].(string); ok {
		return nil, fmt.Errorf("tracker returned failure: %s", reason)
	}

	// parse peers
	compactPeers, ok := dict["peers"].(string)
	if !ok {
		return nil, errors.New("peers field is missing or not a string")
	}

	peers, err := parseCompactPeers(compactPeers)
	if err != nil {
		return nil, err
	}

	// parse interval
	interval, ok := dict["interval"].(int64)
	if !ok {
		return nil, errors.New("interval field is missing or not an integer")
	}

	return &Response{Interval: interval, Peers: peers}, nil
}
