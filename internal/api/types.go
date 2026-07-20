package api

type StatusResponse struct {
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	Completed     int     `json:"completed"`
	Total         int     `json:"total"`
	Pending       int     `json:"pending"`
	ActiveWorkers int     `json:"active_workers"`
	BytesPerSec   float64 `json:"bytes_per_sec"`
	ETASeconds    float64 `json:"eta_seconds"`
	LastError     string  `json:"last_error,omitempty"`
}

type StopResponse struct {
	Result string `json:"result"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
