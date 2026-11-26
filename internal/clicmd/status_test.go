package clicmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestPrintStatusInstalledVersion(t *testing.T) {
	const notice = "已安装 0.5.0，重启 WebMux 后生效，重启会结束所有会话"
	s := contract.StatusV1{V: contract.V, State: contract.StateRunning, Version: "0.4.0", Health: contract.Health{Status: contract.HealthOK}}
	var out bytes.Buffer
	PrintStatus(&out, s)
	if strings.Contains(out.String(), "已安装") {
		t.Fatalf("notice without an installed version:\n%s", out.String())
	}
	s.InstalledVersion = "0.5.0"
	out.Reset()
	PrintStatus(&out, s)
	if !strings.Contains(out.String(), notice+"\n") {
		t.Fatalf("missing %q:\n%s", notice, out.String())
	}
}

func TestPrintStatusSeparatesRelayReadiness(t *testing.T) {
	const origin = "https://host.relay.example"
	status := contract.StatusV1{State: contract.StateRunning, Health: contract.Health{Status: contract.HealthOK},
		Relay: &contract.RelayStatus{
			Enabled: true, Paired: true, State: "failed", Transport: "wss", ControlReady: true,
			BoundPublicURL: origin, Error: "relay origin probe failed",
		},
	}
	var out bytes.Buffer
	PrintStatus(&out, status)
	if text := out.String(); !strings.Contains(text, "baseline=wss, controlReady=true, dataReady=false") ||
		!strings.Contains(text, "relay origin probe failed") || strings.Contains(text, origin) {
		t.Fatalf("management-only Relay: %s", text)
	}
	status.Relay.State, status.Relay.DataReady, status.Relay.PublicURL, status.Relay.Error = "ready", true, origin, ""
	status.URLs.Access = []contract.AccessLink{{Provider: "relay", Label: "Official Relay", URL: origin, Public: true}}
	out.Reset()
	PrintStatus(&out, status)
	if text := out.String(); !strings.Contains(text, "baseline=wss, controlReady=true, dataReady=true") ||
		!strings.Contains(text, origin+" (Official Relay)") || strings.Contains(text, "relay origin probe failed") {
		t.Fatalf("ready Relay: %s", text)
	}
	status.Relay.TransportSelection = &contract.RelayTransportSelection{Preferred: "quic", Pending: "quic", Active: "wss"}
	out.Reset()
	PrintStatus(&out, status)
	if !strings.Contains(out.String(), "baseline=wss,") {
		t.Fatal("unconfirmed preference changed the reported data protocol")
	}
	status.Relay.TransportSelection.Active = "quic"
	out.Reset()
	PrintStatus(&out, status)
	if !strings.Contains(out.String(), "baseline=quic,") {
		t.Fatal("management WSS masked the active QUIC route")
	}
}
