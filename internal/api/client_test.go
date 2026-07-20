package api

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/virajsazzala/swrm/internal/daemon"
)

func TestClientOverUnixSocket(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	state := daemon.NewState("test-torrent")
	state.SetStatus(daemon.StatusCompleted)
	state.SetProgress(daemon.Progress{Completed: 10, Total: 10})

	stopped := make(chan struct{}, 1)
	srv := &http.Server{Handler: NewServer(state, func() { stopped <- struct{}{} })}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	client := NewClient(sockPath)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Status != string(daemon.StatusCompleted) {
		t.Fatalf("Status = %q, want %q", status.Status, daemon.StatusCompleted)
	}
	if status.Completed != 10 || status.Total != 10 {
		t.Fatalf("progress = %+v, want completed=10 total=10", status)
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("expected stop func to be invoked")
	}
}
