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
	Torrent  *torrent.Torrent
	PeerID   [20]byte
	Port     uint16
	Peers    []tracker.Peer
	Interval int64
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

func (d *Downloader) GetReadyPeer() (*peer.Client, error) {
	for _, p := range d.Peers {
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

		return client, nil
	}

	return nil, fmt.Errorf("no usable peers found")
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

	client, err := d.GetReadyPeer()
	if err != nil {
		return fmt.Errorf("failed to get ready peer: %w", err)
	}
	// client.Conn.Close()

	pieceCount := len(d.Torrent.Pieces)
	for pieceIndex := 0; pieceIndex < pieceCount; {
		if !client.Bitfield.HasPiece(pieceIndex) {
			client.Conn.Close()

			client, err = d.GetReadyPeer()
			if err != nil {
				return fmt.Errorf("failed to get ready peer: %w", err)
			}

			continue
		}

		piece, err := client.GetPiece(d.Torrent, pieceIndex)
		if err != nil {
			client.Conn.Close()

			client, err = d.GetReadyPeer()
			if err != nil {
				return fmt.Errorf("failed to get ready peer: %w", err)
			}

			continue
		}

		offset := int64(pieceIndex * d.Torrent.PieceLength)

		_, err = f.WriteAt(piece.Data, offset)
		if err != nil {
			client.Conn.Close()
			return fmt.Errorf("failed to write piece to file: %w", err)
		}

		progress := float64(pieceIndex+1) / float64(pieceCount) * 100
		fmt.Printf("Downloaded piece %d/%d (%.2f%%)\n", pieceIndex+1, pieceCount, progress)

		pieceIndex++
	}

	client.Conn.Close()
	return nil
}
