package api

import (
	"encoding/json"
	"net/http"

	"github.com/virajsazzala/swrm/internal/daemon"
)

func NewServer(state *daemon.State, stop func()) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		snap := state.Snapshot()
		writeJSON(w, http.StatusOK, StatusResponse{
			Name:          snap.Name,
			Status:        string(snap.Status),
			Completed:     snap.Progress.Completed,
			Total:         snap.Progress.Total,
			Pending:       snap.Progress.Pending,
			ActiveWorkers: snap.Progress.ActiveWorkers,
			BytesPerSec:   snap.Progress.BytesPerSec,
			ETASeconds:    snap.Progress.ETASeconds,
			LastError:     snap.LastError,
		})
	})

	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		stop()
		writeJSON(w, http.StatusOK, StopResponse{Result: "stopping"})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}
