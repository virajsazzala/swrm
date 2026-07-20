package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/virajsazzala/swrm/internal/api"
	"github.com/virajsazzala/swrm/internal/daemon"
	"github.com/virajsazzala/swrm/internal/downloader"
	"github.com/virajsazzala/swrm/internal/torrent"
)

func main() {
	torrentPath := flag.String("torrent", "./assets/torrent-files/big-buck-bunny.torrent", "path to the .torrent file")
	outputDir := flag.String("output-dir", ".", "directory to write downloaded files to")
	socketPath := flag.String("socket", defaultSocketPath(), "unix socket path to serve the status/control API on")
	logLevel := flag.String("log-level", "info", "log verbosity: debug, info, warn, error")
	logFormat := flag.String("log-format", "text", "log output format: text, json")
	flag.Parse()

	logger, err := newLogger(*logLevel, *logFormat)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sigCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	ctx, cancel := context.WithCancel(sigCtx)
	defer cancel()

	tor, err := torrent.Open(*torrentPath)
	if err != nil {
		logger.Error("failed to open torrent", "path", *torrentPath, "err", err)
		os.Exit(1)
	}

	dl, err := downloader.New(tor, logger)
	if err != nil {
		logger.Error("failed to initialize downloader", "err", err)
		os.Exit(1)
	}
	dl.OutputDir = *outputDir

	state := daemon.NewState(tor.Name)
	dl.OnProgress = func(p downloader.Progress) {
		state.SetProgress(daemon.Progress{
			Completed:     p.Completed,
			Total:         p.Total,
			Pending:       p.Pending,
			ActiveWorkers: p.ActiveWorkers,
			BytesPerSec:   p.BytesPerSec,
			ETASeconds:    p.ETASeconds,
		})
	}
	dl.OnReconnecting = func(attempt int) {
		state.SetReconnecting(attempt)
	}

	if err := os.MkdirAll(filepath.Dir(*socketPath), 0700); err != nil {
		logger.Error("failed to create socket directory", "socket", *socketPath, "err", err)
		os.Exit(1)
	}

	ln, err := listenUnix(*socketPath)
	if err != nil {
		logger.Error("failed to listen on socket", "socket", *socketPath, "err", err)
		os.Exit(1)
	}

	server := &http.Server{Handler: api.NewServer(state, cancel)}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := daemon.Run(ctx, dl, state); err != nil {
			logger.Error("download failed", "err", err)
			return
		}
		logger.Info("download completed successfully")
	}()

	go func() {
		logger.Info("serving status/control api", "socket", *socketPath)
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("api server error", "err", err)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("api server shutdown error", "err", err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		logger.Warn("timed out waiting for download to exit")
	}

	if err := os.Remove(*socketPath); err != nil && !os.IsNotExist(err) {
		logger.Warn("failed to remove socket file", "err", err)
	}
}

func defaultSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./swrmd.sock"
	}
	return filepath.Join(home, ".swrm", "swrmd.sock")
}

func listenUnix(path string) (net.Listener, error) {
	if _, err := os.Stat(path); err == nil {
		conn, dialErr := net.DialTimeout("unix", path, time.Second)
		if dialErr == nil {
			conn.Close()
			return nil, fmt.Errorf("a daemon is already listening on %s", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("failed to remove stale socket %s: %w", path, err)
		}
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

func newLogger(level, format string) (*slog.Logger, error) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid -log-level %q (want debug, info, warn, or error)", level)
	}

	opts := &slog.HandlerOptions{Level: lvl, AddSource: lvl == slog.LevelDebug}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "text":
		handler = slog.NewTextHandler(os.Stderr, opts)
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		return nil, fmt.Errorf("invalid -log-format %q (want text or json)", format)
	}

	return slog.New(handler), nil
}
