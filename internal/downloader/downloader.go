package downloader

import (
	"context"
	"crypto/sha1"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

const maxConcurrentDials = 50

const maxPieceFailures = 8

type ReconnectConfig struct {
	MaxElapsed      time.Duration
	BaseDelay       time.Duration
	MaxDelay        time.Duration
	JitterFrac      float64
	AnnounceTimeout time.Duration
}

var defaultReconnectConfig = ReconnectConfig{
	MaxElapsed:      10 * time.Minute,
	BaseDelay:       2 * time.Second,
	MaxDelay:        60 * time.Second,
	JitterFrac:      0.3,
	AnnounceTimeout: 90 * time.Second,
}

type Downloader struct {
	Torrent       *torrent.Torrent
	PeerID        [20]byte
	Port          uint16
	OutputDir     string
	Peers         []tracker.Peer
	Workers       []*Worker
	PendingPieces []int
	Interval      int64
	pieceFailures map[int]int
	logger        *slog.Logger
	baseLogger    *slog.Logger
	announced     bool
}

func New(tor *torrent.Torrent, logger *slog.Logger) (*Downloader, error) {
	peerID, err := peer.New()
	if err != nil {
		return nil, fmt.Errorf("error while creating peer id: %w", err)
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Downloader{
		Torrent:    tor,
		PeerID:     peerID,
		Port:       6881,
		OutputDir:  ".",
		logger:     logger.With("component", "downloader"),
		baseLogger: logger,
	}, nil
}

func (d *Downloader) Announce(ctx context.Context) error {
	event := tracker.EventNone
	if !d.announced {
		event = tracker.EventStarted
	}

	resp, err := tracker.Announce(ctx, d.baseLogger.With("component", "tracker"), d.Torrent, d.PeerID, d.Port, event)
	if err != nil {
		return fmt.Errorf("error while announcing: %w", err)
	}

	d.announced = true
	d.Peers = resp.Peers
	d.Interval = resp.Interval

	return nil
}

func (d *Downloader) AnnounceCompleted(ctx context.Context) {
	if !d.announced {
		return
	}
	if _, err := tracker.Announce(ctx, d.baseLogger.With("component", "tracker"), d.Torrent, d.PeerID, d.Port, tracker.EventCompleted); err != nil {
		d.logger.Warn("failed to notify tracker of completion", "err", err)
	}
}

func (d *Downloader) AnnounceStopped(ctx context.Context) {
	if !d.announced {
		return
	}
	if _, err := tracker.Announce(ctx, d.baseLogger.With("component", "tracker"), d.Torrent, d.PeerID, d.Port, tracker.EventStopped); err != nil {
		d.logger.Warn("failed to notify tracker of stop", "err", err)
	}
}

func (d *Downloader) ConnectPeers(ctx context.Context) error {
	d.Workers = nil

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrentDials)

	for i, p := range d.Peers {
		wg.Add(1)
		go func(i int, p tracker.Peer) {
			defer wg.Done()

			addr := fmt.Sprintf("%s:%d", p.IP, p.Port)

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			client, err := peer.Connect(ctx, p, 3*time.Second)
			if err != nil {
				d.logger.Debug("peer dial failed", "peer", addr, "err", err)
				return
			}

			err = client.Handshake(d.Torrent.InfoHash, d.PeerID)
			if err != nil {
				d.logger.Debug("peer handshake failed", "peer", addr, "err", err)
				client.Close()
				return
			}

			client.Start(len(d.Torrent.Pieces))

			err = client.Interested()
			if err != nil {
				d.logger.Debug("peer interested message failed", "peer", addr, "err", err)
				client.Close()
				return
			}

			err = client.WaitForUnchoke(ctx, 30*time.Second)
			if err != nil {
				d.logger.Debug("peer unchoke wait failed", "peer", addr, "err", err)
				client.Close()
				return
			}

			worker := &Worker{
				ID:      i + 1,
				Client:  client,
				Torrent: d.Torrent,
				Jobs:    make(chan downloadJob),
				Results: nil,
				logger:  d.logger.With("worker_id", i+1),
			}

			mu.Lock()
			d.Workers = append(d.Workers, worker)
			mu.Unlock()

			worker.logger.Debug("peer connected", "peer", addr)
		}(i, p)
	}

	wg.Wait()

	if err := ctx.Err(); err != nil {
		return err
	}

	d.logger.Info("peer connection round complete", "connected", len(d.Workers), "attempted", len(d.Peers))

	if len(d.Workers) == 0 {
		return fmt.Errorf("no peers connected")
	}

	return nil
}

func (d *Downloader) reconnectWithBackoff(ctx context.Context, cfg ReconnectConfig, results chan downloadResult) (int, []*Worker, error) {
	deadline := time.Now().Add(cfg.MaxElapsed)
	delay := cfg.BaseDelay
	attempt := 0
	var lastErr error

	for {
		attempt++

		announceCtx, cancel := context.WithTimeout(ctx, cfg.AnnounceTimeout)
		err := d.Announce(announceCtx)
		cancel()

		if err == nil {
			err = d.ConnectPeers(ctx)
		}

		var active int
		var idle []*Worker
		if err == nil {
			active, idle = d.startWorkers(ctx, results)
			if active > 0 {
				return active, idle, nil
			}
			d.closeWorkerList(idle)
			err = fmt.Errorf("connected to %d peer(s) but none hold any pending piece", len(d.Workers))
		}
		lastErr = err

		if err := ctx.Err(); err != nil {
			return 0, nil, err
		}

		if time.Now().After(deadline) {
			return 0, nil, fmt.Errorf("giving up reconnecting after %d attempts: %w", attempt, lastErr)
		}

		wait := delay
		if d.Interval > 0 {
			if floor := time.Duration(d.Interval) * time.Second; floor > wait {
				wait = floor
			}
		}
		wait = addJitter(wait, cfg.JitterFrac)
		if wait > cfg.MaxDelay {
			wait = cfg.MaxDelay
		}

		d.logger.Warn("reconnect attempt failed", "attempt", attempt, "wait", wait.Round(time.Second), "err", lastErr)

		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case <-time.After(wait):
		}

		delay *= 2
	}
}

func addJitter(d time.Duration, frac float64) time.Duration {
	if frac <= 0 {
		return d
	}
	delta := float64(d) * frac
	offset := (rand.Float64()*2 - 1) * delta
	result := d + time.Duration(offset)
	if result < 0 {
		return 0
	}
	return result
}

func (d *Downloader) initializePendingPieces(fw *fileWriter) {
	d.PendingPieces = nil
	d.pieceFailures = make(map[int]int)

	pieceCount := len(d.Torrent.Pieces)
	buf := make([]byte, d.Torrent.PieceLength)
	alreadyComplete := 0

	for i := 0; i < pieceCount; i++ {
		pieceLength := d.Torrent.PieceByteLength(i)
		offset := int64(i) * int64(d.Torrent.PieceLength)

		if err := fw.ReadAt(buf[:pieceLength], offset); err != nil {
			d.PendingPieces = append(d.PendingPieces, i)
			continue
		}

		if sha1.Sum(buf[:pieceLength]) == d.Torrent.Pieces[i] {
			alreadyComplete++
			continue
		}

		d.PendingPieces = append(d.PendingPieces, i)
	}

	if alreadyComplete > 0 {
		d.logger.Info("resumed from existing data", "already_complete", alreadyComplete, "total", pieceCount)
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
	d.closeWorkerList(d.Workers)
}

func (d *Downloader) closeWorkerList(workers []*Worker) {
	for _, w := range workers {
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

func (d *Downloader) startWorkers(ctx context.Context, results chan downloadResult) (int, []*Worker) {
	active := 0
	var idle []*Worker
	for _, worker := range d.Workers {
		worker.Results = results
		go worker.Run(ctx)

		if job, ok := d.nextJob(worker); ok {
			worker.Jobs <- *job
			active++
		} else {
			idle = append(idle, worker)
		}
	}
	return active, idle
}

func (d *Downloader) Download(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fw, err := newFileWriter(d.Torrent, d.OutputDir)
	if err != nil {
		return fmt.Errorf("error creating output files: %w", err)
	}
	defer fw.Close()

	if err := d.ConnectPeers(ctx); err != nil {
		return fmt.Errorf("failed to connect to peers: %w", err)
	}

	defer d.closeWorkers()

	d.initializePendingPieces(fw)

	pieceCount := len(d.Torrent.Pieces)
	completed := pieceCount - len(d.PendingPieces)
	startProgress := math.Round(float64(completed)/float64(pieceCount)*10000) / 100
	lastMilestone := int(startProgress) / 10

	results := make(chan downloadResult)

	activeWorkers, idleWorkers := d.startWorkers(ctx, results)

	d.logger.Info("download started", "pieces", pieceCount, "pending", len(d.PendingPieces), "workers", activeWorkers)

	for completed < pieceCount {
		if activeWorkers == 0 {
			d.closeWorkerList(idleWorkers)
			idleWorkers = nil

			active, idle, err := d.reconnectWithBackoff(ctx, defaultReconnectConfig, results)
			if err != nil {
				return fmt.Errorf("couldn't recover peer connections: %w", err)
			}

			activeWorkers, idleWorkers = active, idle
			continue
		}

		var result downloadResult
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result = <-results:
		}

		activeWorkers--

		if result.Err != nil {
			d.logger.Warn("piece download failed", "worker_id", result.Worker.ID, "piece", result.PieceIndex, "err", result.Err)

			d.removeWorker(result.Worker)

			d.pieceFailures[result.PieceIndex]++
			if d.pieceFailures[result.PieceIndex] > maxPieceFailures {
				return fmt.Errorf("piece %d failed %d times, giving up: %w",
					result.PieceIndex, d.pieceFailures[result.PieceIndex], result.Err)
			}

			d.PendingPieces = append(d.PendingPieces, result.PieceIndex)
		} else {
			offset := int64(result.PieceIndex) * int64(d.Torrent.PieceLength)

			if err := fw.WriteAt(result.Piece.Data, offset); err != nil {
				return fmt.Errorf("failed to write piece: %w", err)
			}

			completed++

			progress := math.Round(float64(completed)/float64(pieceCount)*10000) / 100

			d.logger.Debug("piece downloaded", "piece", result.PieceIndex, "worker_id", result.Worker.ID,
				"completed", completed, "total", pieceCount, "progress_pct", progress)

			if milestone := int(progress) / 10; milestone > lastMilestone {
				lastMilestone = milestone
				d.logger.Info("download progress", "completed", completed, "total", pieceCount,
					"progress_pct", progress, "pending", len(d.PendingPieces), "active_workers", activeWorkers)
			}

			if job, ok := d.nextJob(result.Worker); ok {
				result.Worker.Jobs <- *job
				activeWorkers++
			} else {
				idleWorkers = append(idleWorkers, result.Worker)
			}
		}

		if len(idleWorkers) > 0 {
			var stillIdle []*Worker
			for _, w := range idleWorkers {
				if job, ok := d.nextJob(w); ok {
					w.Jobs <- *job
					activeWorkers++
				} else {
					stillIdle = append(stillIdle, w)
				}
			}
			idleWorkers = stillIdle
		}
	}

	if err := fw.Sync(); err != nil {
		return fmt.Errorf("failed to flush downloaded data to disk: %w", err)
	}

	d.logger.Info("download completed", "pieces", pieceCount)

	return nil
}
