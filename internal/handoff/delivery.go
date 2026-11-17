package handoff

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

func (s *Service) send(ctx context.Context, e *operationEntry) {
	s.mu.Lock()
	p := e.preview
	s.mu.Unlock()
	if p == nil {
		return
	}
	source, sourceErr := s.options.Hybrid.Record(p.SourceSessionID)
	if sourceErr != nil || runtimeIdentity(source) != p.sourceIdentity {
		s.finish(e, "failed", ErrConflict)
		return
	}
	if _, err := s.options.NativeHistory.ScopeDocument(ctx, p.SnapshotID); err != nil {
		s.finish(e, "failed", err)
		return
	}
	config, err := s.options.Launch.Get(p.Request.ConfigID)
	if err != nil {
		s.finish(e, "failed", err)
		return
	}
	fingerprint, err := configFingerprint(config)
	if err != nil || fingerprint != p.configHash {
		s.finish(e, "failed", ErrConflict)
		return
	}
	if p.Request.TargetSessionID != "" {
		budget, err := s.targetBudget(ctx, p.Request.TargetSessionID, p.UserText)
		if err != nil || budget.NativeUsed != p.Budget.NativeUsed || budget.Window != p.Budget.Window || budget.Source != p.Budget.Source {
			s.finish(e, "failed", ErrConflict)
			return
		}
	}
	if p.Request.TargetSessionID != "" {
		target, err := s.options.Hybrid.Record(p.Request.TargetSessionID)
		if err != nil || hybrid.NativeIdentity(target) != p.targetIdentity {
			s.finish(e, "failed", ErrConflict)
			return
		}
	}
	s.mu.Lock()
	if e.operation.FinishedAt != nil {
		s.mu.Unlock()
		return
	}
	if err := ctx.Err(); err != nil {
		s.finishLocked(e, "failed", err.Error())
		s.mu.Unlock()
		return
	}
	e.operation.State = "sending"
	s.mu.Unlock()
	target := p.Request.TargetSessionID
	if target == "" {
		if s.options.Create == nil {
			s.finish(e, "failed", ErrUnavailable)
			return
		}
		in := sessioncreate.Input{ClientRequestID: rand.Text(), ConfigID: &p.Request.ConfigID, Side: string(hybrid.SideACP), Cwd: p.Request.Location.Cwd, AgentConfig: p.Request.AgentConfig}
		location := p.Request.Location
		if location.NewWorktreeBranch != "" {
			in.NewWorktree = &sessioncreate.NewWorktree{RepositoryID: location.RepositoryID, Mode: "new", Branch: location.NewWorktreeBranch, Base: location.NewWorktreeBase}
		} else if location.RepositoryID != "" {
			in.Workspace = &sessioncreate.Reference{RepositoryID: location.RepositoryID, WorktreeID: location.WorktreeID}
		}
		result, err := s.options.Create(ctx, in)
		s.mu.Lock()
		e.operation.Location = result.Location
		if result.Session != nil {
			target = result.Session.Info().ID
			e.operation.TargetSessionID = target
		}
		s.mu.Unlock()
		if err != nil {
			var step *sessioncreate.Error
			if errors.As(err, &step) {
				s.mu.Lock()
				if step.SessionID != "" {
					e.operation.TargetSessionID = step.SessionID
				}
				if step.Location != nil {
					e.operation.Location = step.Location
				}
				s.mu.Unlock()
			}
			if ctx.Err() != nil {
				s.finish(e, "unknown", errors.New("可能已发送，请检查目标会话"))
			} else {
				s.finish(e, "failed", err)
			}
			return
		}
	}
	if err := ctx.Err(); err != nil {
		s.finish(e, "failed", err)
		return
	}
	targetRecord, recordErr := s.options.Hybrid.Record(target)
	if recordErr != nil {
		s.finish(e, "failed", recordErr)
		return
	}
	if len(p.Pages) > 0 {
		capability, err := s.imageCapability(ctx, target)
		if err != nil || capability != "supported" {
			s.finish(e, "failed", errors.New("目标 agent 未确认支持图片，消息未发送；请选择文本模式重新预览"))
			return
		}
	}
	s.mu.Lock()
	s.history[target+"_"+p.SnapshotID] = historyBinding{scopeID: p.SnapshotID, identity: runtimeIdentity(targetRecord)}
	s.mu.Unlock()
	var expectedUsage *hybrid.Usage
	if p.Request.TargetSessionID != "" && p.Budget.Source == "provider" {
		expectedUsage = &hybrid.Usage{Used: p.Budget.NativeUsed, Size: p.Budget.Window}
	}
	expectedIdentity := hybrid.NativeIdentity(targetRecord)
	if p.Request.TargetSessionID != "" {
		expectedIdentity = p.targetIdentity
	}
	input := hybrid.PromptInput{ClientRequestID: rand.Text(), Text: p.Text, TitleSource: p.UserText, Mode: hybrid.ModeRefuse, ExpectedIdentity: expectedIdentity, AwaitAcknowledgement: true, ExpectedUsage: expectedUsage, ImageTextOffset: p.imageTextOffset}
	if p.Format == "image" {
		input.Images = previewImages(p)
	}
	_, err = s.options.Hybrid.Prompt(ctx, target, input)
	if err != nil {
		if errors.Is(err, hybrid.ErrBusy) || errors.Is(err, hybrid.ErrWrongSide) || errors.Is(err, hybrid.ErrSwitching) || errors.Is(err, hybrid.ErrInvalid) || errors.Is(err, hybrid.ErrConflict) || errors.Is(err, hybrid.ErrTooLarge) {
			s.finish(e, "failed", err)
		} else {
			s.finish(e, "unknown", errors.New("可能已发送，请检查目标会话"))
		}
		return
	}
	s.finish(e, "succeeded", nil)
}
