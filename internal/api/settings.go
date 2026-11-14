package api

import (
	"errors"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/wire"
)

func (s *Server) settingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Settings.PruneSessionPins(s.keepSessionPin)
		if err != nil {
			failure(w, settingsStatus(err), err)
			return
		}
		reply(w, 200, wire.SettingsSnapshot(v))
	})
	mux.HandleFunc("PATCH /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var req contract.SettingsPatch
		if !decodeStrict(w, r, &req, 2<<20, settings.ErrInvalid) {
			return
		}
		for field := range req.Changes {
			owner := settingsOwner(field)
			if owner != "" && !s.Config.Plugins.Has(owner) {
				failure(w, http.StatusBadRequest, settings.ErrInvalid)
				return
			}
		}
		changes, err := wire.SettingsChanges(req)
		if err != nil {
			failure(w, 400, err)
			return
		}
		v, err := s.Settings.PatchWithSessionPinFilter(*req.BaseRevision, changes, s.keepSessionPin)
		if err != nil {
			failure(w, settingsStatus(err), err)
			return
		}
		reply(w, 200, wire.SettingsSnapshot(v))
	})
}

// Active core sessions retain their pins; optional archives supply their own
// retention policy without making the settings host import their store.
func (s *Server) keepSessionPin(id string) bool {
	if item, err := s.Manager.Get(id); err == nil && item.Info().Status != session.StatusExited {
		return true
	}
	return s.KeepSessionPin != nil && s.KeepSessionPin(id)
}

func settingsStatus(err error) int {
	if errors.Is(err, settings.ErrInvalid) {
		return http.StatusBadRequest
	}
	if errors.Is(err, settings.ErrConflict) {
		return http.StatusConflict
	}
	if errors.Is(err, settings.ErrUnavailable) {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func settingsOwner(field string) string {
	switch field {
	case "preview", "codeReader":
		return "code"
	case "quickCommands":
		return "quick-commands"
	case "homeWorktrees":
		return "workspace"
	case "ignoreHighPorts":
		return "forward"
	default:
		return ""
	}
}
