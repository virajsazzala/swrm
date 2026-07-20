package daemon

import "sync"

type Status string

const (
	StatusAnnouncing   Status = "announcing"
	StatusDownloading  Status = "downloading"
	StatusReconnecting Status = "reconnecting"
	StatusCompleted    Status = "completed"
	StatusError        Status = "error"
	StatusStopped      Status = "stopped"
)

type Progress struct {
	Completed     int
	Total         int
	Pending       int
	ActiveWorkers int
	BytesPerSec   float64
	ETASeconds    float64
}

type Snapshot struct {
	Name      string
	Status    Status
	Progress  Progress
	LastError string
}

type State struct {
	name string

	mu       sync.Mutex
	status   Status
	progress Progress
	lastErr  string
}

func NewState(name string) *State {
	return &State{name: name, status: StatusAnnouncing, progress: Progress{ETASeconds: -1}}
}

func (s *State) SetStatus(status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *State) SetProgress(p Progress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = p
}

func (s *State) SetReconnecting(attempt int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = StatusReconnecting
}

func (s *State) SetCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = StatusCompleted
}

func (s *State) SetStopped() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = StatusStopped
}

func (s *State) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = StatusError
	if err != nil {
		s.lastErr = err.Error()
	}
}

func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{Name: s.name, Status: s.status, Progress: s.progress, LastError: s.lastErr}
}
