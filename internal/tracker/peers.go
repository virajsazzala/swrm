package tracker

import (
	"encoding/binary"
	"errors"
	"net"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

func parseCompactPeers(s string) ([]Peer, error) {
	p := []byte(s)
	if len(p)%6 != 0 {
		return nil, errors.New("invalid peers list")
	}

	prs := make([]Peer, 0, len(p)/6)
	for i := 0; i < len(p); i += 6 {
		ip := p[i : i+4]
		port := binary.BigEndian.Uint16(p[i+4 : i+6])
		prs = append(prs, Peer{IP: ip, Port: port})
	}

	return prs, nil
}
