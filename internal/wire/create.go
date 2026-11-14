package wire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

func CreateInput(r contract.CreateSessionRequest) sessioncreate.Input {
	in := sessioncreate.Input{ClientRequestID: r.ClientRequestID, Cwd: r.Cwd, Cmd: r.Cmd, Name: r.Name, Env: r.Env, ConfigID: r.ConfigID, Prompt: r.Prompt, UseInitInstruction: r.UseInitInstruction, ExpandPrompt: r.ExpandPrompt, AgentConfig: r.AgentConfig, PermissionMode: string(r.PermissionMode), Cols: r.Cols, Rows: r.Rows}
	switch r.View {
	case contract.ViewChat:
		in.Side = "acp"
	case "", contract.ViewTerminal:
		in.Side = "tui"
	default:
		in.Side = string(r.View)
	}
	if r.Workspace != nil {
		in.Workspace = &sessioncreate.Reference{RepositoryID: r.Workspace.RepositoryID, WorktreeID: r.Workspace.WorktreeID}
	}
	if r.NewWorktree != nil {
		in.NewWorktree = &sessioncreate.NewWorktree{RepositoryID: r.NewWorktree.RepositoryID, Mode: r.NewWorktree.Mode, Branch: r.NewWorktree.Branch, Base: r.NewWorktree.Base, Path: r.NewWorktree.Path}
	}
	if r.Attachments != nil {
		in.Attachments = make([]string, len(r.Attachments))
		for i, at := range r.Attachments {
			in.Attachments[i] = at.Path
		}
	}
	return in
}
func CreateResult(r sessioncreate.Result, info session.Info) contract.CreateSessionResult {
	warnings := r.ConfigWarnings
	if warnings == nil {
		warnings = []string{}
	}
	out := contract.CreateSessionResult{Session: Session(info), ConfigWarnings: warnings}
	if r.InitialInput != nil {
		receipt := SessionInputResult(*r.InitialInput)
		out.InitialInput = &receipt
	}
	return out
}
