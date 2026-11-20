package contract

// PerfClientConfig is the read-only performance collection hint returned in
// GET /api/config when [perf] recording is enabled on the server.
type PerfClientConfig struct {
	Enabled bool `json:"enabled"`
	SlowMS  int  `json:"slowMs"`
}

// PerfEventBatch is the body of POST /api/perf/events from browser clients.
// It carries only sanitized timing and resource metadata, never terminal I/O,
// prompt text, or credentials.
type PerfEventBatch struct {
	Events []PerfEventInput `json:"events"`
}

// PerfEventInput is one client-side bottleneck measurement.
type PerfEventInput struct {
	TS         string  `json:"ts,omitempty"`
	Kind       string  `json:"kind"`
	Op         string  `json:"op"`
	DurationMS float64 `json:"durationMs,omitempty"`
	ServerMS   float64 `json:"serverMs,omitempty"`
	Value      float64 `json:"value,omitempty"`
	Bytes      int64   `json:"bytes,omitempty"`
	Status     int     `json:"status,omitempty"`
	Route      string  `json:"route,omitempty"`
	Detail     string  `json:"detail,omitempty"`
}
