package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/virajsazzala/swrm/internal/daemon"
)

func getStatus(t *testing.T, url string) StatusResponse {
	t.Helper()
	resp, err := http.Get(url + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func TestServerStatus(t *testing.T) {
	state := daemon.NewState("test-torrent")
	state.SetStatus(daemon.StatusDownloading)
	state.SetProgress(daemon.Progress{
		Completed:     5,
		Total:         10,
		Pending:       5,
		ActiveWorkers: 2,
		BytesPerSec:   1234.5,
		ETASeconds:    10,
	})

	srv := httptest.NewServer(NewServer(state, func() {}))
	defer srv.Close()

	status := getStatus(t, srv.URL)

	if status.Status != string(daemon.StatusDownloading) {
		t.Errorf("Status = %q, want %q", status.Status, daemon.StatusDownloading)
	}
	if status.Completed != 5 || status.Total != 10 || status.Pending != 5 {
		t.Errorf("progress = %+v, want completed=5 total=10 pending=5", status)
	}
	if status.ActiveWorkers != 2 {
		t.Errorf("ActiveWorkers = %d, want 2", status.ActiveWorkers)
	}
	if status.BytesPerSec != 1234.5 {
		t.Errorf("BytesPerSec = %v, want 1234.5", status.BytesPerSec)
	}
}

func TestServerStop(t *testing.T) {
	state := daemon.NewState("test-torrent")
	called := make(chan struct{}, 1)

	srv := httptest.NewServer(NewServer(state, func() { called <- struct{}{} }))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /stop: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	select {
	case <-called:
	default:
		t.Fatal("expected stop func to be invoked")
	}
}

func TestServerStatusWrongMethodRejected(t *testing.T) {
	state := daemon.NewState("test-torrent")
	srv := httptest.NewServer(NewServer(state, func() {}))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/status", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
