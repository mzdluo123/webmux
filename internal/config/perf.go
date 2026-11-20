package config

import "fmt"

// PerfConfig controls opt-in local performance profiling and slow-operation
// recording. Disabled by default so ordinary installs incur zero disk writes.
type PerfConfig struct {
	Enabled        bool `toml:"enabled"`
	SlowMS         int  `toml:"slow_ms"`
	SampleSeconds  int  `toml:"sample_seconds"`
	MaxBytesMB     int  `toml:"max_bytes_mb"`
	RetentionDays  int  `toml:"retention_days"`
	FlightRecorder bool `toml:"flight_recorder"`
	SpikeMS        int  `toml:"spike_ms"`
}

// DefaultPerfConfig returns the safe defaults for [perf].
func DefaultPerfConfig() PerfConfig {
	return PerfConfig{
		Enabled:        false,
		SlowMS:         100,
		SampleSeconds:  10,
		MaxBytesMB:     50,
		RetentionDays:  7,
		FlightRecorder: false,
		SpikeMS:        500,
	}
}

func (p *PerfConfig) validate() error {
	def := DefaultPerfConfig()
	if p.SlowMS == 0 {
		p.SlowMS = def.SlowMS
	}
	if p.SampleSeconds == 0 {
		p.SampleSeconds = def.SampleSeconds
	}
	if p.MaxBytesMB == 0 {
		p.MaxBytesMB = def.MaxBytesMB
	}
	if p.RetentionDays == 0 {
		p.RetentionDays = def.RetentionDays
	}
	if p.SpikeMS == 0 {
		p.SpikeMS = def.SpikeMS
	}
	if p.SlowMS < 10 || p.SlowMS > 60000 {
		return fmt.Errorf("perf.slow_ms %d must be between 10 and 60000", p.SlowMS)
	}
	if p.SampleSeconds < 1 || p.SampleSeconds > 3600 {
		return fmt.Errorf("perf.sample_seconds %d must be between 1 and 3600", p.SampleSeconds)
	}
	if p.MaxBytesMB < 1 || p.MaxBytesMB > 1024 {
		return fmt.Errorf("perf.max_bytes_mb %d must be between 1 and 1024", p.MaxBytesMB)
	}
	if p.RetentionDays < 1 || p.RetentionDays > 90 {
		return fmt.Errorf("perf.retention_days %d must be between 1 and 90", p.RetentionDays)
	}
	if p.SpikeMS < 50 || p.SpikeMS > 60000 {
		return fmt.Errorf("perf.spike_ms %d must be between 50 and 60000", p.SpikeMS)
	}
	return nil
}
