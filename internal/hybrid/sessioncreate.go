package hybrid

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

// Creation contributes chat startup to the ordinary session creation pipeline.
func (s *Service) Creation() sessioncreate.Chat { return creationAdapter{s} }

type creationAdapter struct{ service *Service }

func (a creationAdapter) Check(ctx context.Context, in sessioncreate.Input, command sessioncreate.Command, cwd string) error {
	if err := a.service.Unavailable(); err != nil {
		return err
	}
	if !command.Chat {
		return ErrNoChatView
	}
	return a.service.CheckCreate(ctx, CreateInput{Cwd: cwd, ConfigID: command.ConfigID, Cmd: command.Cmd, Env: command.Env, StartSide: Side(in.Side), Cols: in.Cols, Rows: in.Rows, Attention: command.Attention})
}

func (a creationAdapter) Create(ctx context.Context, in sessioncreate.Input, command sessioncreate.Command, cwd string, initial func(string) (string, error)) (*session.Session, bool, error) {
	sent := false
	item, err := a.service.Create(ctx, CreateInput{Cwd: cwd, Name: in.Name, ConfigID: command.ConfigID, Cmd: command.Cmd, Env: command.Env, StartSide: Side(in.Side), Cols: in.Cols, Rows: in.Rows, Attention: command.Attention, InitialPrompt: initial, InitialPromptSent: &sent})
	return item, sent, err
}

func (a creationAdapter) Prompt(ctx context.Context, id, requestID, text, title string, paths []string) (session.InputResult, error) {
	attachments := make([]Attachment, len(paths))
	for i, path := range paths {
		attachments[i] = Attachment{Path: path}
	}
	if requestID == "" {
		requestID = "first-" + id
	}
	result, err := a.service.Prompt(ctx, id, PromptInput{ClientRequestID: requestID, Text: text, TitleSource: title, Attachments: attachments})
	return session.InputResult{Side: string(SideACP), Prompt: session.InputPromptResult(result)}, err
}

// LiveSession returns only an existing runtime shell; handoff recovery never
// manufactures a second shell for an already-active durable reservation.
func (s *Service) LiveSession(id string) (*session.Session, error) { return s.manager.Get(id) }

func (a creationAdapter) Configure(ctx context.Context, id string, in sessioncreate.Input) ([]string, error) {
	warnings := []string{}
	current, err := a.service.Settings(ctx, id)
	if err != nil {
		return warnings, err
	}
	warn := func(text string) error { warnings = append(warnings, text); return a.service.Notice(ctx, id, text) }
	requestID := in.ClientRequestID
	if requestID == "" {
		requestID = id
	}
	optionIDs := make([]string, 0, len(in.AgentConfig))
	for optionID := range in.AgentConfig {
		optionIDs = append(optionIDs, optionID)
	}
	slices.Sort(optionIDs)
	for _, optionID := range optionIDs {
		want := in.AgentConfig[optionID]
		i := slices.IndexFunc(current.Config, func(o ConfigOption) bool { return o.ID == optionID })
		if i < 0 {
			if err := warn(fmt.Sprintf("配置项 %s 不可用，未应用 %s", optionID, want)); err != nil {
				return warnings, err
			}
			continue
		}
		option := current.Config[i]
		set := ConfigInput{ClientRequestID: creationRequestID(requestID, "config", optionID), ConfigID: optionID}
		if option.Enabled != nil {
			enabled, err := strconv.ParseBool(want)
			if err != nil || (want != "true" && want != "false") {
				if err := warn(fmt.Sprintf("配置项 %s 只接受 true 或 false，未应用 %s", option.Name, want)); err != nil {
					return warnings, err
				}
				continue
			}
			if enabled == *option.Enabled {
				continue
			}
			set.Enabled = &enabled
		} else {
			if want == option.Value {
				continue
			}
			set.Value = &want
		}
		err := a.service.SetConfig(ctx, id, set)
		switch {
		case err == nil:
		case errors.Is(err, ErrInvalid), errors.Is(err, ErrAgentCall):
			if err := warn(fmt.Sprintf("配置项 %s 未能设为 %s：%v", option.Name, want, err)); err != nil {
				return warnings, err
			}
		default:
			return warnings, err
		}
	}
	if mode := PermissionMode(in.PermissionMode); mode != "" && mode != current.PermissionMode {
		if err := a.service.SetPermissionMode(ctx, id, PermissionModeInput{ClientRequestID: creationRequestID(requestID, "permission", ""), Mode: mode}); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

func creationRequestID(request, command, item string) string {
	sum := sha256.Sum256([]byte(request + "\x00" + command + "\x00" + item))
	return "qs-" + hex.EncodeToString(sum[:16])
}
