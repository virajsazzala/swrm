package downloader

import (
	"fmt"
	"os"
	"time"

	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

type Downloader struct {
	Torrent        *torrent.Torrent
	PeerID         [20]byte
	Port           uint16
	Peers          []tracker.Peer
	ConnectedPeers []*peer.Client
	Interval       int64
}

func New(tor *torrent.Torrent) (*Downloader, error) {
	peerID, err := peer.New()
	if err != nil {
		return nil, fmt.Errorf("error while creating peer id: %w", err)
	}

	return &Downloader{Torrent: tor, PeerID: peerID, Port: 6881}, nil
}

func (d *Downloader) Announce() error {
	resp, err := tracker.Announce(d.Torrent, d.PeerID, d.Port)
	if err != nil {
		return fmt.Errorf("error while announcing: %w", err)
	}

	d.Peers = resp.Peers
	d.Interval = resp.Interval

	return nil
}

func (d *Downloader) ConnectPeers() error {
	d.ConnectedPeers = nil
	for i, p := range d.Peers {
		client, err := peer.Connect(p, 3*time.Second)
		if err != nil {
			// maybe log not able to connect here, later.
			continue
		}

		err = client.Handshake(d.Torrent.InfoHash, d.PeerID)
		if err != nil {
			client.Conn.Close()
			continue
		}

		err = client.Interested()
		if err != nil {
			client.Conn.Close()
			continue
		}

		err = client.WaitForUnchoke()
		if err != nil {
			client.Conn.Close()
			continue
		}

		d.ConnectedPeers = append(d.ConnectedPeers, client)

		fmt.Printf("Connected to peer %d\n", i+1)
	}

	fmt.Printf("Total connected peers %d/%d\n", len(d.ConnectedPeers), len(d.Peers))

	if len(d.ConnectedPeers) == 0 {
		return fmt.Errorf("no peers connected")
	}

	return nil
}

func (d *Downloader) removeConnectedPeer(peer *peer.Client) {
	for i, p := range d.ConnectedPeers {
		if p == peer {
			peer.Conn.Close()
			d.ConnectedPeers = append(d.ConnectedPeers[:i], d.ConnectedPeers[i+1:]...)
		}
	}
}

func (d *Downloader) closeConnectedPeers() {
	for _, p := range d.ConnectedPeers {
		p.Conn.Close()
	}
}

func (d *Downloader) findPeerWithPiece(pieceIndex int) (*peer.Client, error) {
	for _, c := range d.ConnectedPeers {
		if c.Bitfield.HasPiece(pieceIndex) {
			return c, nil
		}
	}

	return nil, fmt.Errorf("no connected peer has the piece %d", pieceIndex)
}

func (d *Downloader) Download() error {
	f, err := os.OpenFile(d.Torrent.Name, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer f.Close()

	err = f.Truncate(d.Torrent.Length)
	if err != nil {
		return fmt.Errorf("error pre-allocating file size: %w", err)
	}

	if err := d.ConnectPeers(); err != nil {
		return fmt.Errorf("failed to connect to peers: %w", err)
	}

	defer d.closeConnectedPeers()

	pieceCount := len(d.Torrent.Pieces)
	for pieceIndex := 0; pieceIndex < pieceCount; {
		client, err := d.findPeerWithPiece(pieceIndex)
		if err != nil {
			return fmt.Errorf("failed to get peer with piece %d: %w", pieceIndex, err)
		}

		piece, err := client.GetPiece(d.Torrent, pieceIndex)
		if err != nil {
			d.removeConnectedPeer(client)
			continue
		}

		offset := int64(pieceIndex * d.Torrent.PieceLength)

		_, err = f.WriteAt(piece.Data, offset)
		if err != nil {
			return fmt.Errorf("failed to write piece to file: %w", err)
		}

		progress := float64(pieceIndex+1) / float64(pieceCount) * 100
		fmt.Printf("Downloaded piece %d/%d (%.2f%%)\n", pieceIndex+1, pieceCount, progress)

		pieceIndex++
	}

	return nil
}
