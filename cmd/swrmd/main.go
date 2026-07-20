package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/virajsazzala/swrm/internal/downloader"
	"github.com/virajsazzala/swrm/internal/torrent"
)

func main() {
	torrentPath := flag.String("torrent", "./assets/torrent-files/big-buck-bunny.torrent", "path to the .torrent file")
	logLevel := flag.String("log-level", "info", "log verbosity: debug, info, warn, error")
	logFormat := flag.String("log-format", "text", "log output format: text, json")
	flag.Parse()

	logger, err := newLogger(*logLevel, *logFormat)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	if err := dl.Announce(ctx); err != nil {
		logger.Error("announce failed", "err", err)
		os.Exit(1)
	}

	downloadErr := dl.Download(ctx)

	notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if downloadErr != nil {
		dl.AnnounceStopped(notifyCtx)
		logger.Error("download failed", "err", downloadErr)
		os.Exit(1)
	}

	dl.AnnounceCompleted(notifyCtx)
	logger.Info("download completed successfully")
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
