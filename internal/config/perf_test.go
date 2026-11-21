package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPerfConfiguration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	for _, row := range []struct {
		name, body string
		err        string
		want       PerfConfig
	}{
		{name: "defaults", want: DefaultPerfConfig()},
		{
			name: "enabled partial",
			body: "enabled = true\nflight_recorder = true\n",
			want: PerfConfig{Enabled: true, SlowMS: 100, SampleSeconds: 10, MaxBytesMB: 50, RetentionDays: 7, FlightRecorder: true, SpikeMS: 500},
		},
		{
			name: "custom valid",
			body: "enabled = true\nslow_ms = 80\nsample_seconds = 5\nmax_bytes_mb = 100\nretention_days = 14\nflight_recorder = true\nspike_ms = 400\n",
			want: PerfConfig{Enabled: true, SlowMS: 80, SampleSeconds: 5, MaxBytesMB: 100, RetentionDays: 14, FlightRecorder: true, SpikeMS: 400},
		},
		{name: "slow_ms too low", body: "slow_ms = 5\n", err: "perf.slow_ms"},
		{name: "sample_seconds too high", body: "sample_seconds = 9999\n", err: "perf.sample_seconds"},
		{name: "max_bytes_mb too high", body: "max_bytes_mb = 2048\n", err: "perf.max_bytes_mb"},
		{name: "retention_days too high", body: "retention_days = 120\n", err: "perf.retention_days"},
		{name: "spike_ms too low", body: "spike_ms = 10\n", err: "perf.spike_ms"},
	} {
		t.Run(row.name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(row.name, " ", "-")+".toml")
			content := ""
			if row.body != "" {
				content = "[perf]\n" + row.body
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			c, err := Load(path, true)
			if err != nil {
				t.Fatal(err)
			}
			err = c.Validate()
			if row.err != "" {
				if err == nil || !strings.Contains(err.Error(), row.err) {
					t.Fatalf("Validate() = %v, want %q", err, row.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.Perf != row.want {
				t.Fatalf("Perf = %+v, want %+v", c.Perf, row.want)
			}
		})
	}
}
