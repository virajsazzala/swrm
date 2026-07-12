package peer

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/virajsazzala/swrm/internal/tracker"
)

type Client struct {
	Conn   net.Conn
	PeerID [20]byte
}

func Connect(peer tracker.Peer, to time.Duration) (*Client, error) {
	d := net.Dialer{Timeout: to}
	addr := net.JoinHostPort(peer.IP.String(), strconv.FormatUint(uint64(peer.Port), 10))
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	return &Client{Conn: conn}, nil
}
