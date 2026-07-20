package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Downloaderish interface {
	Announce(ctx context.Context) error
	Download(ctx context.Context) error
	AnnounceCompleted(ctx context.Context)
	AnnounceStopped(ctx context.Context)
}

func Run(ctx context.Context, dl Downloaderish, state *State) error {
	state.SetStatus(StatusAnnouncing)
	if err := dl.Announce(ctx); err != nil {
		state.SetError(err)
		return fmt.Errorf("announce failed: %w", err)
	}

	state.SetStatus(StatusDownloading)

	var downloadErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				downloadErr = fmt.Errorf("download panicked: %v", r)
			}
		}()
		downloadErr = dl.Download(ctx)
	}()

	notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if downloadErr != nil {
		dl.AnnounceStopped(notifyCtx)
		if errors.Is(downloadErr, context.Canceled) {
			state.SetStopped()
		} else {
			state.SetError(downloadErr)
		}
		return downloadErr
	}

	dl.AnnounceCompleted(notifyCtx)
	state.SetCompleted()
	return nil
}
