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

func New(t *torrent.Torrent) (*Downloader, error) {
	p, err := peer.New()
	if err != nil {
		return nil, fmt.Errorf("error while creating peer id: %w", err)
	}

	return &Downloader{Torrent: t, PeerID: p, Port: 6881}, nil
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
		con, err := peer.Connect(p, 3*time.Second)
		if err != nil {
			// maybe log not able to connect here, later.
			continue
		}

		err = con.Handshake(d.Torrent.InfoHash, d.PeerID)
		if err != nil {
			con.Conn.Close()
			continue
		}

		err = con.Interested()
		if err != nil {
			con.Conn.Close()
			continue
		}

		err = con.WaitForUnchoke()
		if err != nil {
			con.Conn.Close()
			continue
		}

		return con, nil
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

	rp, err := d.GetReadyPeer()
	if err != nil {
		return fmt.Errorf("failed to get ready peer: %w", err)
	}
	rp.Conn.Close()

	tp := len(d.Torrent.Pieces)
	for cp := 0; cp < tp; {
		if !rp.Bitfield.HasPiece(cp) {
			rp.Conn.Close()

			rp, err = d.GetReadyPeer()
			if err != nil {
				return fmt.Errorf("failed to get ready peer: %w", err)
			}

			continue
		}

		pi, err := rp.GetPiece(d.Torrent, cp)
		if err != nil {
			rp.Conn.Close()

			rp, err = d.GetReadyPeer()
			if err != nil {
				return fmt.Errorf("failed to get ready peer: %w", err)
			}

			continue
		}

		oset := int64(cp * d.Torrent.PieceLength)

		_, err = f.WriteAt(pi.Data, oset)
		if err != nil {
			rp.Conn.Close()
			return fmt.Errorf("failed to write torrent block: %w", err)
		}

		pct := float64(cp+1) / float64(tp) * 100
		fmt.Printf("Downloaded piece %d/%d (%.2f%%)\n", cp+1, tp, pct)

		cp++
	}

	rp.Conn.Close()
	return nil
}
