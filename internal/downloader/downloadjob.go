package downloader

import (
	"fmt"
	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
)

type downloadJob struct {
	PieceIndex int
}

type downloadResult struct {
	Worker     *Worker
	PieceIndex int
	Piece      *peer.Piece
	Err        error
}

type Worker struct {
	ID      int
	Client  *peer.Client
	Torrent *torrent.Torrent
	Jobs    chan downloadJob
	Results chan<- downloadResult
}

func (w *Worker) Run() {
	for job := range w.Jobs {

		fmt.Printf("[Worker %02d] downloading piece %d\n", w.ID, job.PieceIndex)
		piece, err := w.Client.GetPiece(w.Torrent, job.PieceIndex)
		fmt.Printf("[Worker %02d] finished piece %d\n", w.ID, job.PieceIndex)

		w.Results <- downloadResult{Worker: w, PieceIndex: job.PieceIndex, Piece: piece, Err: err}
	}
}
