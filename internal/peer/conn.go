package peer

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/virajsazzala/swrm/internal/tracker"
)

type Client struct {
	Conn     net.Conn
	PeerID   [20]byte
	Bitfield Bitfield
	Choked   bool
	Interest bool
}

func Connect(peer tracker.Peer, timeout time.Duration) (*Client, error) {
	dialer := net.Dialer{Timeout: timeout}

	addr := net.JoinHostPort(
		peer.IP.String(),
		strconv.FormatUint(uint64(peer.Port), 10),
	)

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	return &Client{Conn: conn}, nil
}
