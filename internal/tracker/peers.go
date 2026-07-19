package tracker

import (
	"encoding/binary"
	"errors"
	"net"
)

/* might move this to internal/peer/peer.go TBD*/
type Peer struct {
	IP   net.IP
	Port uint16
}

func parseCompactPeers(s string) ([]Peer, error) {
	data := []byte(s)
	if len(data)%6 != 0 {
		return nil, errors.New("invalid peers list")
	}

	peers := make([]Peer, 0, len(data)/6)
	for i := 0; i < len(data); i += 6 {
		ip := data[i : i+4]
		port := binary.BigEndian.Uint16(data[i+4 : i+6])
		peers = append(peers, Peer{IP: ip, Port: port})
	}

	return peers, nil
}
