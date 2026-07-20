package daemon

import (
	"context"
	"errors"
	"testing"

	"github.com/virajsazzala/swrm/internal/downloader"
)

var _ Downloaderish = (*downloader.Downloader)(nil)

type fakeDownloader struct {
	announceErr    error
	downloadErr    error
	downloadPanic  bool
	completedCalls int
	stoppedCalls   int
}

func (f *fakeDownloader) Announce(ctx context.Context) error { return f.announceErr }

func (f *fakeDownloader) Download(ctx context.Context) error {
	if f.downloadPanic {
		panic("boom")
	}
	return f.downloadErr
}

func (f *fakeDownloader) AnnounceCompleted(ctx context.Context) { f.completedCalls++ }
func (f *fakeDownloader) AnnounceStopped(ctx context.Context)   { f.stoppedCalls++ }

func TestRunSuccess(t *testing.T) {
	fd := &fakeDownloader{}
	state := NewState("test-torrent")

	if err := Run(context.Background(), fd, state); err != nil {
		t.Fatalf("Run: %v", err)
	}

	snap := state.Snapshot()
	if snap.Status != StatusCompleted {
		t.Fatalf("status = %q, want %q", snap.Status, StatusCompleted)
	}
	if fd.completedCalls != 1 {
		t.Fatalf("completedCalls = %d, want 1", fd.completedCalls)
	}
	if fd.stoppedCalls != 0 {
		t.Fatalf("stoppedCalls = %d, want 0", fd.stoppedCalls)
	}
}

func TestRunAnnounceFailure(t *testing.T) {
	fd := &fakeDownloader{announceErr: errors.New("tracker unreachable")}
	state := NewState("test-torrent")

	err := Run(context.Background(), fd, state)
	if err == nil {
		t.Fatal("expected an error")
	}

	snap := state.Snapshot()
	if snap.Status != StatusError {
		t.Fatalf("status = %q, want %q", snap.Status, StatusError)
	}
	if snap.LastError == "" {
		t.Fatal("expected LastError to be set")
	}
	if fd.completedCalls != 0 || fd.stoppedCalls != 0 {
		t.Fatalf("completedCalls=%d stoppedCalls=%d, want both 0", fd.completedCalls, fd.stoppedCalls)
	}
}

func TestRunDownloadFailure(t *testing.T) {
	fd := &fakeDownloader{downloadErr: errors.New("piece 4 failed 8 times")}
	state := NewState("test-torrent")

	err := Run(context.Background(), fd, state)
	if err == nil {
		t.Fatal("expected an error")
	}

	snap := state.Snapshot()
	if snap.Status != StatusError {
		t.Fatalf("status = %q, want %q", snap.Status, StatusError)
	}
	if fd.stoppedCalls != 1 {
		t.Fatalf("stoppedCalls = %d, want 1", fd.stoppedCalls)
	}
	if fd.completedCalls != 0 {
		t.Fatalf("completedCalls = %d, want 0", fd.completedCalls)
	}
}

func TestRunCanceledMidDownloadReportsStopped(t *testing.T) {
	fd := &fakeDownloader{downloadErr: context.Canceled}
	state := NewState("test-torrent")

	err := Run(context.Background(), fd, state)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	snap := state.Snapshot()
	if snap.Status != StatusStopped {
		t.Fatalf("status = %q, want %q", snap.Status, StatusStopped)
	}
	if fd.stoppedCalls != 1 {
		t.Fatalf("stoppedCalls = %d, want 1", fd.stoppedCalls)
	}
}

func TestRunDownloadPanicIsRecovered(t *testing.T) {
	fd := &fakeDownloader{downloadPanic: true}
	state := NewState("test-torrent")

	err := Run(context.Background(), fd, state)
	if err == nil {
		t.Fatal("expected an error from the recovered panic")
	}

	snap := state.Snapshot()
	if snap.Status != StatusError {
		t.Fatalf("status = %q, want %q", snap.Status, StatusError)
	}
	if fd.stoppedCalls != 1 {
		t.Fatalf("stoppedCalls = %d, want 1", fd.stoppedCalls)
	}
}
