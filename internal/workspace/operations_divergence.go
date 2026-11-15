package workspace

import (
	"context"
	"fmt"
	"os"
)

func (s *Service) replayDivergenceOperation(ctx context.Context, repositoryID, worktreeID string, op GitOperation) (DivergenceResult, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return DivergenceResult{}, err
	}
	switch op.Status {
	case GitOpSucceeded, GitOpNeedsReview:
		readCtx, cancel := context.WithTimeout(ctx, readTimeout)
		defer cancel()
		loc, locErr := s.locateEditableWorktree(readCtx, repositoryID, worktreeID)
		var inProg InProgressOperation
		if locErr == nil {
			if lErr := s.acquireWorktreeWrite(readCtx, loc.Path); lErr == nil {
				inProg, _ = s.inspectInProgressLocked(readCtx, loc)
				s.releaseWorktreeWrite(loc.Path)
			}
		} else {
			inProg = InProgressOperation{
				WorktreeID:       worktreeID,
				ConflictedFiles:  []ConflictFileSummary{},
				ContinueBlockers: []Issue{},
			}
		}
		var tree Worktree
		if aErr := acquire(readCtx, r); aErr == nil {
			if t, tErr := s.currentTree(readCtx, r, worktreeID); tErr == nil {
				tree = s.withLastCommit(readCtx, r, t)
			}
			release(r)
		}
		outcome := DivergenceOutcomeCompleted
		switch {
		case op.Kind == GitOpAbort:
			outcome = DivergenceOutcomeAborted
		case op.Status == GitOpNeedsReview && op.FailureKind == "conflicted":
			outcome = DivergenceOutcomeConflicted
		}
		return DivergenceResult{
			OperationID: op.ID,
			Kind:        op.Kind,
			Outcome:     outcome,
			Summary:     op.Summary,
			HeadOID:     firstNonEmpty(op.ActualOID, inProg.HeadOID),
			Subject:     op.Subject,
			InProgress:  inProg,
			Worktree:    tree,
		}, nil
	case GitOpUncertain:
		return DivergenceResult{}, problemf(ErrUncertain, "%s", op.Summary)
	default:
		return DivergenceResult{}, replayFailureError(op)
	}
}

func (s *Service) reconcileDivergenceOperation(ctx context.Context, r *repositoryState, op *GitOperation) {
	wtPath := op.Preview.WorktreePath
	wtReachable := false
	if wtPath != "" {
		if info, err := os.Stat(wtPath); err == nil && info.IsDir() {
			wtReachable = true
		}
	}
	if !wtReachable {
		op.Status = GitOpUncertain
		op.Summary = "服务重启时工作树不可达，请刷新并检查当前分支与冲突状态"
		return
	}

	loc := WorktreeLocation{
		RepositoryID: op.RepositoryID,
		WorktreeID:   op.WorktreeID,
		CommonDir:    r.entry.CommonDir,
		Path:         wtPath,
		Available:    true,
	}
	inProg, inErr := s.inspectInProgressLocked(ctx, loc)
	if inErr != nil {
		op.Status = GitOpUncertain
		op.Summary = "服务重启时无法读取工作树操作标记，请刷新核对"
		return
	}

	if op.Kind == GitOpAbort {
		if !inProg.Active && op.Preview.TargetOID != "" && inProg.HeadOID == op.Preview.TargetOID {
			op.Status = GitOpSucceeded
			op.ActualOID = inProg.HeadOID
			op.Summary = fmt.Sprintf("服务重启对账确认中止操作已完成，已恢复至 %s", shortOID(inProg.HeadOID))
			return
		}
		op.Status = GitOpUncertain
		op.ActualOID = inProg.HeadOID
		op.Summary = "服务重启时中止操作未能唯一确认完成，请检查当前进行中状态"
		return
	}

	if inProg.Active || inProg.ConflictedCount > 0 {
		op.Status = GitOpNeedsReview
		op.FailureKind = "conflicted"
		op.ActualOID = inProg.HeadOID
		op.Summary = fmt.Sprintf("服务重启对账检测到进行中的 %s 操作（%d 个冲突文件），请继续解决冲突或中止", inProg.Kind, inProg.ConflictedCount)
		op.ReviewReasons = []string{op.Summary}
		return
	}

	reflogTag := fmt.Sprintf("webmux-%s:%s", op.Kind, op.ID)
	matches, _ := s.scanCommitReflogWithBranch(ctx, wtPath, true, r.entry.AnchorPath, op.Preview.Branch, reflogTag)
	if len(matches) > 0 {
		op.Status = GitOpSucceeded
		op.ActualOID = inProg.HeadOID
		op.Subject = inProg.HeadSubject
		op.Summary = fmt.Sprintf("服务重启对账确认 %s 操作已完成（HEAD %s）", op.Kind, shortOID(inProg.HeadOID))
		return
	}

	if op.Preview.HeadOID != "" && inProg.HeadOID == op.Preview.HeadOID && !inProg.Active {
		op.Status = GitOpFailed
		op.FailureKind = "conflict"
		op.Summary = fmt.Sprintf("服务在 %s 写入前中断，对账确认 HEAD 与工作树操作状态未变化", op.Kind)
		return
	}

	op.Status = GitOpUncertain
	op.ActualOID = inProg.HeadOID
	op.Summary = fmt.Sprintf("服务重启时无法唯一确认 %s 操作最终状态，请刷新提交历史核对", op.Kind)
}
