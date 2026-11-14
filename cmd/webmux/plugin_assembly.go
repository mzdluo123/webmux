package main

import (
	"log/slog"

	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/files"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/handoff"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/hybridresume"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/p2p"
	"github.com/rainchan/webmux/internal/p2psettings"
	"github.com/rainchan/webmux/internal/push"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/tailscalesettings"
	"github.com/rainchan/webmux/internal/tunnelsettings"
	"github.com/rainchan/webmux/internal/voice"
	"github.com/rainchan/webmux/internal/voicesettings"
	"github.com/rainchan/webmux/internal/workspace"
)

// pluginServices is an explicit composition-root record, not a service lookup
// container. A disabled plugin has no service instance or lifetime to release.
type pluginServices struct {
	launch         *launch.Service
	workspace      *workspace.Service
	code           *codeview.Service
	files          *files.Service
	artifacts      *artifacts.Service
	creation       *sessioncreate.Service
	recovery       *hybridresume.Service
	chat           *hybrid.Service
	conversations  *conversation.Service
	handoff        *handoff.Service
	push           *push.Service
	forward        *forward.Service
	voice          *voice.Service
	voiceStore     *voicesettings.Service
	voiceControl   *voiceController
	p2p            *p2p.Service
	p2pStore       *p2psettings.Service
	p2pControl     *p2pController
	tunnel         *tunnelController
	tunnelStore    *tunnelsettings.Service
	tunnelPath     string
	tailscale      *tailscaleController
	tailscaleStore *tailscalesettings.Service
}

// stop joins producers before the processes and stores they can still use.
// It is also used on partially initialized startup failures.
func (p *pluginServices) stop() {
	if p.creation != nil {
		p.creation.Close()
	}
	if p.tunnel != nil {
		p.tunnel.Close()
	}
	if p.tailscale != nil {
		p.tailscale.Close()
	}
	if p.p2p != nil {
		p.p2p.Close()
	}
	if p.forward != nil {
		p.forward.Close()
	}
	if p.push != nil {
		p.push.Close()
	}
	if p.handoff != nil {
		p.handoff.Close()
	}
	if p.chat != nil {
		p.chat.Close()
	}
}

// release runs after the core PTYs and connection handlers have stopped.
func (p *pluginServices) release() {
	if p.code != nil {
		p.code.Close()
	}
	if p.workspace != nil {
		p.workspace.Close()
	}
	if p.artifacts != nil {
		p.artifacts.Close()
	}
	if p.conversations != nil {
		if err := p.conversations.Close(); err != nil {
			slog.Warn("conversation database close failed")
		}
	}
	if p.launch != nil {
		p.launch.Close()
	}
	if p.voiceStore != nil {
		p.voiceStore.Close()
	}
	if p.p2pStore != nil {
		p.p2pStore.Close()
	}
	if p.tunnelStore != nil {
		p.tunnelStore.Close()
	}
	if p.tailscaleStore != nil {
		p.tailscaleStore.Close()
	}
}
