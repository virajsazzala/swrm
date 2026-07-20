package daemon

import (
	"errors"
	"sync"
	"testing"
)

func TestStateConcurrentAccess(t *testing.T) {
	s := NewState("test-torrent")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.SetProgress(Progress{Completed: i, Total: 100})
			s.SetStatus(StatusDownloading)
			_ = s.Snapshot()
		}(i)
	}
	wg.Wait()

	snap := s.Snapshot()
	if snap.Status != StatusDownloading {
		t.Fatalf("status = %q, want %q", snap.Status, StatusDownloading)
	}
	if snap.Progress.Total != 100 {
		t.Fatalf("progress.Total = %d, want 100", snap.Progress.Total)
	}
}

func TestStateTransitions(t *testing.T) {
	s := NewState("test-torrent")

	if got := s.Snapshot().Status; got != StatusAnnouncing {
		t.Fatalf("initial status = %q, want %q", got, StatusAnnouncing)
	}

	s.SetReconnecting(3)
	if got := s.Snapshot().Status; got != StatusReconnecting {
		t.Fatalf("status after SetReconnecting = %q, want %q", got, StatusReconnecting)
	}

	s.SetCompleted()
	if got := s.Snapshot().Status; got != StatusCompleted {
		t.Fatalf("status after SetCompleted = %q, want %q", got, StatusCompleted)
	}

	s.SetStopped()
	if got := s.Snapshot().Status; got != StatusStopped {
		t.Fatalf("status after SetStopped = %q, want %q", got, StatusStopped)
	}

	wantErr := "boom"
	s.SetError(errors.New(wantErr))
	snap := s.Snapshot()
	if snap.Status != StatusError {
		t.Fatalf("status after SetError = %q, want %q", snap.Status, StatusError)
	}
	if snap.LastError != wantErr {
		t.Fatalf("LastError = %q, want %q", snap.LastError, wantErr)
	}
}
