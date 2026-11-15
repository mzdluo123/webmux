package clicmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// configFields are the flags add and edit share.
type configFields struct {
	icon                      string
	env                       envFlag
	chat                      bool
	previewLines, previewSkip int
	set                       map[string]bool
}

func bindConfigFields(fs *flag.FlagSet, f *configFields) {
	f.env = envFlag{}
	fs.StringVar(&f.icon, "icon", "", "the launch picker `ICON` key (for example claudecode, codex, terminal)")
	fs.Var(f.env, "env", "set environment variable `NAME=VALUE` for sessions started from the config")
	fs.BoolVar(&f.chat, "chat", false, "enable the chat view for a supported agent, beside its terminal")
	fs.IntVar(&f.previewLines, "preview-lines", 0, "show `N` preview lines on the home page (1-50) instead of the global default")
	fs.IntVar(&f.previewSkip, "preview-skip", 0, "skip the last `N` lines of the preview (0-50)")
}

// visited records which flags the command line set.
func visited(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

// checkChat asks the server whether argv can have a chat view.
func (e Env) checkChat(ctx context.Context, client *localclient.Client, argv []string, snapshot contract.Capabilities) (int, bool) {
	if err := localclient.RequireSnapshotCapability(snapshot, "chat.acp"); err != nil {
		return e.Fail(err), false
	}
	var support contract.ChatSupport
	if err := client.Do(ctx, http.MethodPost, "/api/launch-configs/chat-support", contract.ChatSupportRequest{Cmd: argv}, &support); err != nil {
		return e.Fail(err), false
	}
	if !support.Supported {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: this command cannot have a chat view (%s); supported agents: %s\n",
			support.Reason, strings.Join(support.SupportedAgents, ", "))
		return contract.ExitError, false
	}
	if support.Connector != nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: its chat view needs %s (install: %s)\n", support.Connector.Title, support.Connector.InstallCommand)
	}
	return contract.ExitOK, true
}

func (e Env) applyPreview(input *contract.LaunchConfigWrite, f configFields) (int, bool) {
	if !f.set["preview-lines"] && !f.set["preview-skip"] {
		return contract.ExitOK, true
	}
	preview := contract.PreviewSettings{Lines: 10, Skip: 1}
	if input.Preview != nil {
		preview = *input.Preview
	}
	if f.set["preview-lines"] {
		preview.Lines = f.previewLines
	}
	if f.set["preview-skip"] {
		preview.Skip = f.previewSkip
	}
	if preview.Lines < 1 || preview.Lines > 50 || preview.Skip < 0 || preview.Skip > 50 {
		return e.Usagef("--preview-lines is 1-50 and --preview-skip 0-50"), false
	}
	input.Preview = &preview
	return contract.ExitOK, true
}

func (e Env) printSaved(config contract.LaunchConfig, asJSON bool) int {
	if asJSON {
		for name := range config.Env {
			config.Env[name] = hiddenValue
		}
		writeJSON(e.Stdout, config)
	} else {
		_, _ = fmt.Fprintln(e.Stdout, config.ID)
	}
	return contract.ExitOK
}
