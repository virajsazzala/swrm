package peer

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/virajsazzala/swrm/internal/tracker"
)

const readIdleTimeout = 2*time.Minute + 30*time.Second

type Client struct {
	Conn       net.Conn
	PeerID     [20]byte
	Interest   bool
	mu         sync.Mutex
	bitfield   Bitfield
	choked     bool
	pieceCount int
	readErr    error
	blocks     chan *Block
	notify     chan struct{}
	closeCh    chan struct{}
	closeOnce  sync.Once
}

func Connect(ctx context.Context, peer tracker.Peer, timeout time.Duration) (*Client, error) {
	dialer := net.Dialer{Timeout: timeout}
	addr := net.JoinHostPort(peer.IP.String(), strconv.FormatUint(uint64(peer.Port), 10))

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	return &Client{
		Conn:    conn,
		choked:  true,
		closeCh: make(chan struct{}),
		blocks:  make(chan *Block, 4*pipelineDepth),
		notify:  make(chan struct{}, 1),
	}, nil
}

func (c *Client) Start(pieceCount int) {
	c.pieceCount = pieceCount
	go c.readLoop()
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		c.Conn.Close()
	})
}

func (c *Client) signal() {
	select {
	case c.notify <- struct{}{}:
	default:
	}
}

func (c *Client) HasPiece(pieceIndex int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bitfield.HasPiece(pieceIndex)
}

func (c *Client) IsChoked() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.choked
}

func (c *Client) setChoked(v bool) {
	c.mu.Lock()
	c.choked = v
	c.mu.Unlock()
	c.signal()
}

func (c *Client) setBitfield(bf Bitfield) {
	c.mu.Lock()
	c.bitfield = bf
	c.mu.Unlock()
}

func (c *Client) setPiece(pieceIndex int) {
	c.mu.Lock()
	if c.bitfield == nil {
		c.bitfield = make(Bitfield, (c.pieceCount+7)/8)
	}
	c.bitfield.SetPiece(pieceIndex)
	c.mu.Unlock()
}

func (c *Client) setReadErr(err error) {
	c.mu.Lock()
	c.readErr = err
	c.mu.Unlock()
}

func (c *Client) ReadErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readErr
}
