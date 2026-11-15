package featureapi

import (
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/featurewire"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/transport"
	"github.com/rainchan/webmux/internal/workspace"
)

func (s *Server) launchRoutes(mux transport.Registrar) {
	const base = "/api/launch-configs"
	serve(mux, "GET "+base, http.StatusOK, launchFailure, func(*http.Request) ([]contract.LaunchConfigSummary, error) {
		list, err := s.Launch.List()
		return wire.LaunchSummaries(list, s.chatOptions), err
	})
	serve(mux, "GET "+base+"/presets", http.StatusOK, launchFailure, func(*http.Request) ([]contract.LaunchPreset, error) {
		list, err := s.Launch.Presets()
		return wire.LaunchPresets(list), err
	})
	serveIn(mux, "POST "+base+"/presets", http.StatusOK, launchFailure, func(_ *http.Request, req contract.RestoreLaunchPresetsRequest) ([]contract.LaunchConfigSummary, error) {
		list, err := s.Launch.RestorePresets(req.Name)
		return wire.LaunchSummaries(list, s.chatOptions), err
	})
	serve(mux, "GET "+base+"/{id}", http.StatusOK, launchFailure, func(r *http.Request) (contract.LaunchConfig, error) {
		item, err := s.Launch.Get(r.PathValue("id"))
		return wire.LaunchConfig(item), err
	})
	serveIn(mux, "POST "+base, http.StatusCreated, launchFailure, func(_ *http.Request, req contract.LaunchConfigWrite) (contract.LaunchConfig, error) {
		input, err := s.launchWrite(req, nil)
		if err != nil {
			return contract.LaunchConfig{}, err
		}
		item, err := s.Launch.Create(input)
		return wire.LaunchConfig(item), err
	})
	serveIn(mux, "PUT "+base+"/{id}", http.StatusOK, launchFailure, func(r *http.Request, req contract.LaunchConfigWrite) (contract.LaunchConfig, error) {
		previous, err := s.Launch.Get(r.PathValue("id"))
		if err != nil {
			return contract.LaunchConfig{}, err
		}
		input, err := s.launchWrite(req, &previous)
		if err != nil {
			return contract.LaunchConfig{}, err
		}
		item, err := s.Launch.Update(r.PathValue("id"), input)
		return wire.LaunchConfig(item), err
	})
	serveDone(mux, "DELETE "+base+"/{id}", launchFailure, func(r *http.Request) error { return s.Launch.Delete(r.PathValue("id")) })
}

// Chat-specific launch preparation is a chat+launch contribution, not a launch
// dependency on Workspace. A repository is consulted only when requested.
func (s *Server) launchChatRoutes(mux transport.Registrar) {
	const base = "/api/launch-configs"
	serveIn(mux, "POST "+base+"/chat-support", http.StatusOK, launchFailure, func(_ *http.Request, req contract.ChatSupportRequest) (contract.ChatSupport, error) {
		return wire.ChatSupport(s.Launch.HybridSupport(req.Cmd)), nil
	})
	serveIn(mux, "POST "+base+"/{id}/options", http.StatusOK, sessionFailure, func(r *http.Request, req contract.LaunchOptionsRequest) ([]contract.ChatConfig, error) {
		command, err := s.Launch.Resolve(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		if !command.Chat {
			return nil, hybrid.ErrNoChatView
		}
		cwd := s.Config.DefaultCwd
		if req.RepositoryID != "" {
			if !s.Config.Features.Has("workspace") || s.Workspace == nil {
				return nil, workspace.ErrUnavailable
			}
			repositories, err := s.Workspace.List(r.Context())
			if err != nil {
				return nil, err
			}
			found := false
			for _, repository := range repositories {
				if repository.ID == req.RepositoryID {
					cwd = repository.AnchorPath
					found = true
					break
				}
			}
			if !found {
				return nil, workspace.ErrNotFound
			}
		}
		options, err := s.Hybrid.ProbeAgentOptions(r.Context(), hybrid.CreateInput{Cwd: cwd, ConfigID: command.ConfigID, Cmd: command.Cmd, Env: command.Env})
		return wire.ChatConfigs(options), err
	})
}

func (s *Server) launchWrite(req contract.LaunchConfigWrite, previous *launch.Config) (launch.Input, error) {
	input := wire.LaunchInput(req)
	if !s.Config.Features.Has("chat") {
		if req.Chat != nil {
			return launch.Input{}, launch.ErrInvalid
		}
		if previous != nil {
			input.Hybrid = previous.Hybrid
		}
	}
	if !s.Config.Features.Has("notifications") {
		if req.Attention != nil {
			return launch.Input{}, launch.ErrInvalid
		}
		if previous != nil {
			input.Attention = previous.Attention
		}
	}
	return input, nil
}

func (s *Server) chatOptions(id string) []hybrid.ConfigOption {
	if !s.Config.Features.Has("chat") || s.Hybrid == nil {
		return nil
	}
	options, _ := s.Hybrid.AgentOptions(id)
	return options
}