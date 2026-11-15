package pluginapi

import (
	"errors"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/pluginwire"
)

func (s *Server) EnrichConfig(view *contract.ServerConfig) {
	if s.Config.Plugins.Has("workspace") && s.Workspace != nil {
		view.Workspace = pluginwire.Capability(s.Workspace.Capability())
	}
	view.PortDetection = s.Config.Plugins.Has("forward") && forward.ListeningPortsSupported
	view.Voice = s.Config.Plugins.Has("voice") && s.Voice != nil && s.Voice.Enabled()
	view.P2P = s.Config.Plugins.Has("p2p") && s.P2P != nil && s.P2P.Enabled()
	if s.Config.Plugins.Has("cloudflare") {
		if s.TunnelSettings != nil {
			settings := s.TunnelSettings.Settings()
			view.Tunnel.Named, view.Tunnel.Hostname = settings.Mode == "named", settings.Hostname
		}
		if s.Tunnel != nil {
			status := s.Tunnel()
			view.Tunnel.State = status.State
			if view.Tunnel.State == "" {
				view.Tunnel.State = contract.TunnelOff
			}
			view.Tunnel.PublicURL = status.PublicURL
			view.Tunnel.Enabled = view.Tunnel.State == contract.TunnelStarting || view.Tunnel.State == contract.TunnelReady
		}
	}
}

func (s *Server) Renamed(id, name string) error {
	if s.Hybrid == nil {
		return nil
	}
	err := s.Hybrid.SetName(id, name)
	if errors.Is(err, hybrid.ErrNotFound) || (!s.Hybrid.HasChatView(id) && errors.Is(err, hybrid.ErrUnavailable)) {
		return nil
	}
	return err
}

func (s *Server) KeepSessionPin(id string) bool {
	if s.Hybrid == nil {
		return false
	}
	record, err := s.Hybrid.Record(id)
	return errors.Is(err, hybrid.ErrUnavailable) || (err == nil && record.State != hybrid.StateClosed)
}

func LocalTunnelFailure(w http.ResponseWriter, err error) { localTunnelFailure(w, err) }
