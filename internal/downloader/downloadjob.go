package downloader

import (
	"context"
	"log/slog"

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
	logger  *slog.Logger
}

func (w *Worker) Run(ctx context.Context) {
	for job := range w.Jobs {
		w.logger.Debug("downloading piece", "piece", job.PieceIndex)
		piece, err := w.Client.GetPiece(ctx, w.Torrent, job.PieceIndex)
		w.logger.Debug("finished piece", "piece", job.PieceIndex)

		select {
		case w.Results <- downloadResult{Worker: w, PieceIndex: job.PieceIndex, Piece: piece, Err: err}:
		case <-ctx.Done():
			return
		}
	}
}
