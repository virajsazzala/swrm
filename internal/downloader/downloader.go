package downloader

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

type Downloader struct {
	Torrent       *torrent.Torrent
	PeerID        [20]byte
	Port          uint16
	Peers         []tracker.Peer
	Workers       []*Worker
	PendingPieces []int
	Interval      int64
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
	d.Workers = nil

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, p := range d.Peers {
		wg.Add(1)
		go func(i int, p tracker.Peer) {
			defer wg.Done()

			client, err := peer.Connect(p, 3*time.Second)
			if err != nil {
				// maybe log not able to connect here, later.
				return
			}

			err = client.Handshake(d.Torrent.InfoHash, d.PeerID)
			if err != nil {
				client.Conn.Close()
				return
			}

			client.Start(len(d.Torrent.Pieces))

			err = client.Interested()
			if err != nil {
				client.Conn.Close()
				return
			}

			err = client.WaitForUnchoke(30 * time.Second)
			if err != nil {
				client.Conn.Close()
				return
			}

			worker := &Worker{
				ID:      i + 1,
				Client:  client,
				Torrent: d.Torrent,
				Jobs:    make(chan downloadJob),
				Results: nil,
			}

			mu.Lock()
			d.Workers = append(d.Workers, worker)
			mu.Unlock()

			fmt.Printf("Worker %d connected\n", worker.ID)
		}(i, p)
	}

	wg.Wait()

	if len(d.Workers) == 0 {
		return fmt.Errorf("no peers connected")
	}

	return nil
}

func (d *Downloader) initializePendingPieces() {
	d.PendingPieces = make([]int, len(d.Torrent.Pieces))

	for i := range d.PendingPieces {
		d.PendingPieces[i] = i
	}
}

func (d *Downloader) nextJob(worker *Worker) (*downloadJob, bool) {
	for i, piece := range d.PendingPieces {
		if !worker.Client.HasPiece(piece) {
			continue
		}

		d.PendingPieces = append(d.PendingPieces[:i], d.PendingPieces[i+1:]...)

		return &downloadJob{PieceIndex: piece}, true
	}

	return nil, false
}

func (d *Downloader) closeWorkers() {
	for _, w := range d.Workers {
		close(w.Jobs)
		w.Client.Close()
	}
}

func (d *Downloader) removeWorker(worker *Worker) {
	worker.Client.Close()
	close(worker.Jobs)

	for i, w := range d.Workers {
		if w == worker {
			d.Workers = append(d.Workers[:i], d.Workers[i+1:]...)
			return
		}
	}
}

func (d *Downloader) startWorkers(results chan downloadResult) int {
	active := 0
	for _, worker := range d.Workers {
		worker.Results = results
		go worker.Run()

		if job, ok := d.nextJob(worker); ok {
			worker.Jobs <- *job
			active++
		}
	}
	return active
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

	defer d.closeWorkers()

	d.initializePendingPieces()

	results := make(chan downloadResult, len(d.Workers))

	activeWorkers := d.startWorkers(results)

	completed := 0
	pieceCount := len(d.Torrent.Pieces)

	for completed < pieceCount && activeWorkers > 0 {
		result := <-results
		activeWorkers--

		if result.Err != nil {
			fmt.Printf("worker %d failed piece %d: %v\n", result.Worker.ID, result.PieceIndex, result.Err)

			d.removeWorker(result.Worker)
			d.PendingPieces = append(d.PendingPieces, result.PieceIndex)

			if len(d.Workers) == 0 {
				if err := d.Announce(); err != nil {
					return fmt.Errorf("couldn't find any peers while reannounce: %w", err)
				}

				if err := d.ConnectPeers(); err != nil {
					return fmt.Errorf("couldn't connect to new peers: %w", err)
				}

				activeWorkers += d.startWorkers(results)
			}

			continue
		}

		offset := int64(result.PieceIndex) * int64(d.Torrent.PieceLength)

		_, err = f.WriteAt(result.Piece.Data, offset)
		if err != nil {
			return fmt.Errorf("failed to writing piece: %w", err)
		}

		completed++

		progress := float64(completed) / float64(pieceCount) * 100
		fmt.Printf("Downloaded piece %d (%d/%d, %.2f%%)\n", result.PieceIndex, completed, pieceCount, progress)
		fmt.Printf("Pending=%d Active=%d Completed=%d\n", len(d.PendingPieces), activeWorkers, completed)

		job, ok := d.nextJob(result.Worker)
		if ok {
			result.Worker.Jobs <- *job
			activeWorkers++
		}
	}

	if completed != pieceCount {
		return fmt.Errorf("download incomplete")
	}

	return nil
}
