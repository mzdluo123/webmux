package workspace

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"time"
)

type (
	beforeOperationGitHookKey struct{}
	afterOperationGitHookKey  struct{}
)

var errSimulatedCrash = problemf(ErrInternal, "simulated process crash before ledger finalization")

func shouldAbortBeforeGit(ctx context.Context, opID string) bool {
	if hook, ok := ctx.Value(beforeOperationGitHookKey{}).(func(string) bool); ok && hook != nil {
		return !hook(opID)
	}
	return false
}

func shouldAbortAfterGit(ctx context.Context, opID string) bool {
	if hook, ok := ctx.Value(afterOperationGitHookKey{}).(func(string) bool); ok && hook != nil {
		return !hook(opID)
	}
	return false
}

func runIdempotentOperation[T any](
	ctx context.Context,
	s *Service,
	repositoryID, clientRequestID, kind, requestHash string,
	replay func(GitOperation) (T, error),
	execute func(opID string) (T, error),
) (T, error) {
	var zero T
	if err := validateClientRequestID(clientRequestID); err != nil {
		return zero, err
	}
	if clientRequestID == "" {
		opID, err := randomID()
		if err != nil {
			return zero, err
		}
		return execute(opID)
	}

	opID, err := randomID()
	if err != nil {
		return zero, err
	}
	key := repositoryID + "\x00" + clientRequestID

	s.opsMu.Lock()
	if !s.opsAvailable {
		errText := s.opsError
		s.opsMu.Unlock()
		return zero, problemf(ErrUnavailable, "%s", errText)
	}
	if flight, ok := s.opFlights[key]; ok {
		if flight.requestHash != requestHash || flight.kind != kind {
			s.opsMu.Unlock()
			return zero, problemf(ErrIdempotency, "clientRequestId 已被不同请求参数占用")
		}
		s.opsMu.Unlock()
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-flight.done:
			if flight.err != nil {
				return zero, flight.err
			}
			res, _ := flight.result.(T)
			return res, nil
		}
	}
	for i := len(s.operations) - 1; i >= 0; i-- {
		op := s.operations[i]
		if op.RepositoryID == repositoryID && op.ClientRequestID == clientRequestID {
			if op.RequestHash != requestHash || op.Kind != kind {
				s.opsMu.Unlock()
				return zero, problemf(ErrIdempotency, "clientRequestId 已被不同请求参数占用")
			}
			copied := cloneGitOperation(op)
			s.opsMu.Unlock()
			return replay(copied)
		}
	}
	flight := &opFlight{
		done:        make(chan struct{}),
		opID:        opID,
		requestHash: requestHash,
		kind:        kind,
	}
	s.opFlights[key] = flight
	s.opsMu.Unlock()

	res, execErr := execute(opID)
	s.opsMu.Lock()
	flight.result = res
	flight.err = execErr
	delete(s.opFlights, key)
	close(flight.done)
	s.opsMu.Unlock()
	return res, execErr
}

func classifyFailureKind(err error) string {
	switch {
	case err == nil:
		return ""
	case isSubkind(err, ErrLocked):
		return "locked"
	case isSubkind(err, ErrHookFailed):
		return "hook_failed"
	case isSubkind(err, ErrStale):
		return "stale"
	case isSubkind(err, ErrBlocked):
		return "blocked"
	case isSubkind(err, ErrConflicted):
		return "conflicted"
	case isSubkind(err, ErrReadOnly):
		return "read_only"
	}
	switch errorKind(err) {
	case ErrTimeout:
		return "timeout"
	case ErrUnavailable:
		return "unavailable"
	case ErrInvalid:
		return "invalid"
	case ErrNotFound:
		return "not_found"
	case ErrConflict:
		return "conflict"
	default:
		return "internal"
	}
}

func isSubkind(err, target error) bool {
	if err == nil {
		return false
	}
	var we *Error
	if e, ok := err.(*Error); ok {
		we = e
	}
	return we != nil && we.subkind == target
}

func replayFailureError(op GitOperation) error {
	msg := op.Summary
	if msg == "" {
		msg = "上次 Git 操作执行失败"
	}
	switch op.FailureKind {
	case "locked":
		return problemf(ErrLocked, "%s", msg)
	case "hook_failed":
		return problemf(ErrHookFailed, "%s", msg)
	case "stale":
		return problemf(ErrStale, "%s", msg)
	case "blocked":
		return problemf(ErrBlocked, "%s", msg)
	case "conflicted":
		return problemf(ErrConflicted, "%s", msg)
	case "timeout":
		return problemf(ErrTimeout, "%s", msg)
	case "unavailable":
		return problemf(ErrUnavailable, "%s", msg)
	case "invalid":
		return problemf(ErrInvalid, "%s", msg)
	case "not_found":
		return problemf(ErrNotFound, "%s", msg)
	default:
		return problemf(ErrConflict, "%s", msg)
	}
}

func (s *Service) reconcileOperationsOnStartup() {
	s.opsSaveMu.Lock()
	defer s.opsSaveMu.Unlock()
	s.opsMu.Lock()
	if !s.opsAvailable || len(s.operations) == 0 {
		s.opsMu.Unlock()
		return
	}
	next := cloneGitOperations(s.operations)
	s.opsMu.Unlock()

	byRepo := make(map[string]*repositoryState, len(s.entries))
	for _, r := range s.entries {
		byRepo[r.entry.ID] = r
	}

	dirty := false
	now := time.Now().UTC()
	for i := range next {
		if isTerminalOpStatus(next[i].Status) {
			continue
		}
		dirty = true
		s.reconcileSingleOperation(&next[i], byRepo[next[i].RepositoryID], now)
	}
	if !dirty {
		return
	}
	if err := s.commitOperationsLocked(next); err != nil {
		slog.Warn("failed to persist reconciled git operations", "err", err)
	}
}

func (s *Service) reconcileSingleOperation(op *GitOperation, r *repositoryState, now time.Time) {
	op.FinishedAt = &now
	if s.gitPath == "" || !s.capability.RegistryAvailable {
		op.Status = GitOpUncertain
		op.Summary = "服务重启时 Git 或仓库清单不可用，无法自动对账，请人工核对仓库状态"
		return
	}
	if r == nil {
		op.Status = GitOpUncertain
		op.Summary = "服务重启时仓库已不在清单中，无法自动对账"
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	if err := s.verifyIdentity(ctx, r); err != nil {
		op.Status = GitOpUncertain
		op.Summary = "服务重启时仓库不可达或身份已变化，请人工核对仓库状态"
		return
	}

	switch op.Kind {
	case GitOpCommit:
		s.reconcileCommitOperation(ctx, r, op)
	case GitOpPush:
		s.reconcilePushOperation(ctx, r, op)
	case GitOpFastForward:
		s.reconcileFastForwardOperation(ctx, r, op)
	case GitOpSwitch:
		s.reconcileSwitchOperation(ctx, r, op)
	case GitOpMerge, GitOpRebase, GitOpCherryPick, GitOpRevert, GitOpContinue, GitOpAbort:
		s.reconcileDivergenceOperation(ctx, r, op)
	case GitOpFetch:
		op.Status = GitOpUncertain
		op.Summary = "服务重启时远程获取未完成确认，可重新获取远程刷新状态"
	case GitOpStashPush, GitOpStashApply, GitOpStashPop, GitOpStashDrop,
		GitOpBranchRename, GitOpBranchUpstream,
		GitOpTagCreate, GitOpTagDelete,
		GitOpRemoteAdd, GitOpRemoteUpdate, GitOpRemoteRemove:
		s.reconcileV2Operation(ctx, r, op)
	default:
		op.Status = GitOpUncertain
		op.Summary = "服务重启时操作状态未知，请刷新核对"
	}
}

func (s *Service) reconcileCommitOperation(ctx context.Context, r *repositoryState, op *GitOperation) {
	reflogTag := "webmux-commit:" + op.ID
	wtPath := op.Preview.WorktreePath
	wtReachable := false
	if wtPath != "" {
		if info, err := os.Stat(wtPath); err == nil && info.IsDir() {
			wtReachable = true
		}
	}
	readCwd := r.entry.AnchorPath
	if wtReachable {
		readCwd = wtPath
	}

	matches, reflogErr := s.scanCommitReflogWithBranch(ctx, wtPath, wtReachable, r.entry.AnchorPath, op.Preview.Branch, reflogTag)
	if len(matches) > 0 {
		actualOID := matches[0]
		meta, metaErr := s.readCommitFullMeta(ctx, readCwd, actualOID)
		if metaErr != nil {
			op.Status = GitOpUncertain
			op.ActualOID = actualOID
			op.Summary = "服务重启对账已定位提交对象，但读取提交元数据失败，请刷新历史核对"
			op.ReviewReasons = []string{op.Summary}
			return
		}
		expectedParents := op.Preview.ExpectedParents
		if !op.Preview.Amend && expectedParents == nil && op.Preview.HeadOID != "" {
			expectedParents = []string{op.Preview.HeadOID}
		}
		reasons := make([]string, 0)
		if len(matches) > 1 {
			reasons = append(reasons, "同次提交过程中检测到多次引用更新（例如 post-commit 修正了提交）")
		}
		if op.Preview.ExpectedTreeOID != "" && meta.TreeOID != op.Preview.ExpectedTreeOID {
			reasons = append(reasons, "实际生成的提交树摘要与预览不一致（可能由 pre-commit 等钩子修改并重新暂存了文件）")
		}
		if !slices.Equal(meta.Parents, expectedParents) {
			reasons = append(reasons, "实际提交的父节点列表与预览不一致")
		}
		if wtReachable {
			if latestHead, _ := s.resolveCommitReadOnly(ctx, wtPath, "HEAD"); latestHead != "" && latestHead != actualOID {
				if !s.isAncestorReadOnly(ctx, wtPath, actualOID, latestHead) {
					reasons = append(reasons, "提交完成后当前分支 HEAD 被后续改写（如 post-commit --amend）")
				}
			}
		}
		op.ActualOID = actualOID
		op.ActualTreeOID = meta.TreeOID
		op.Parents = meta.Parents
		op.Subject = meta.Subject
		op.AuthorName = meta.AuthorName
		op.AuthorEmail = meta.AuthorEmail
		op.CommitterName = meta.CommitterName
		op.CommitterEmail = meta.CommitterEmail
		op.ReviewReasons = reasons
		if len(reasons) > 0 {
			op.Status = GitOpNeedsReview
			op.Summary = "服务重启对账确认提交已创建，但存在钩子改写或树差异，请复核"
		} else {
			op.Status = GitOpSucceeded
			op.Summary = "服务重启对账确认提交已完成"
		}
		s.clearWorktreeCommitDraftBestEffort(op.RepositoryID, op.WorktreeID)
		return
	}

	if wtReachable {
		latestHead, hErr := s.resolveCommitReadOnly(ctx, wtPath, "HEAD")
		branch, detached, symErr := s.symbolicRefHead(ctx, wtPath)
		sameBranch := !detached && branch == op.Preview.Branch
		unbornUnchanged := op.Preview.HeadState == "unborn" && op.Preview.HeadOID == "" && latestHead == "" && sameBranch
		bornUnchanged := reflogErr == nil && op.Preview.HeadOID != "" && latestHead == op.Preview.HeadOID && sameBranch
		if hErr == nil && symErr == nil && (unbornUnchanged || bornUnchanged) {
			op.Status = GitOpFailed
			op.FailureKind = "conflict"
			op.Summary = "服务在提交写入前中断，对账确认 HEAD 未变化且未产生新提交"
			return
		}
	}
	op.Status = GitOpUncertain
	op.Summary = "服务重启时无法唯一确认提交是否已写入，请先刷新并检查提交历史与暂存区"
	op.ReviewReasons = []string{op.Summary}
}

func (s *Service) reconcilePushOperation(ctx context.Context, r *repositoryState, op *GitOperation) {
	anchor := r.entry.AnchorPath
	if op.Preview.Remote != "" && op.Preview.Target != "" && op.Preview.TargetOID != "" {
		trackingRef := "refs/remotes/" + op.Preview.Remote + "/" + op.Preview.Target
		tip, err := s.resolveCommit(ctx, anchor, trackingRef)
		if err == nil && (tip == op.Preview.TargetOID || s.isAncestorReadOnly(ctx, anchor, op.Preview.TargetOID, tip)) {
			if op.Preview.Publish && op.Preview.Branch != "" {
				rem, _, rErr := s.configValue(ctx, anchor, "branch."+op.Preview.Branch+".remote")
				merge, _, mErr := s.configValue(ctx, anchor, "branch."+op.Preview.Branch+".merge")
				if rErr != nil || mErr != nil || rem != op.Preview.Remote || merge != "refs/heads/"+op.Preview.Target {
					op.Status = GitOpUncertain
					op.ActualOID = tip
					op.Summary = "服务重启对账发现远端跟踪引用已指向目标提交，但分支上游配置尚未写入，请刷新核对"
					return
				}
			}
			op.Status = GitOpSucceeded
			op.RemoteStatus = remoteOK
			op.ActualOID = op.Preview.TargetOID
			op.Summary = "服务重启对账确认远端跟踪引用已包含目标提交"
			return
		}
	}
	op.Status = GitOpUncertain
	op.Summary = "服务重启时推送结果无法仅凭本地引用确认，请先获取远程并核对远端分支状态"
}

func (s *Service) reconcileFastForwardOperation(ctx context.Context, r *repositoryState, op *GitOperation) {
	anchor := r.entry.AnchorPath
	if op.Preview.Branch != "" && op.Preview.TargetOID != "" {
		tip, err := s.resolveCommit(ctx, anchor, "refs/heads/"+op.Preview.Branch)
		if err == nil && tip == op.Preview.TargetOID {
			if op.WorktreeID != "" {
				tree, tErr := s.currentTree(ctx, r, op.WorktreeID)
				if tErr != nil || !tree.Available || tree.Operation != nil || tree.Head == nil || *tree.Head != op.Preview.TargetOID ||
					tree.Branch == nil || *tree.Branch != op.Preview.Branch || tree.Changes == nil || tree.Changes.Conflicted > 0 {
					op.Status = GitOpUncertain
					op.ActualOID = tip
					op.Summary = "服务重启对账发现分支引用已移动，但工作树状态需人工核对"
					return
				}
			}
			op.Status = GitOpSucceeded
			op.ActualOID = tip
			op.Summary = "服务重启对账确认分支已完成快进"
			return
		}
	}
	op.Status = GitOpUncertain
	op.Summary = "服务重启时快进操作被中断，请刷新并检查分支与工作树状态"
}

func (s *Service) reconcileSwitchOperation(ctx context.Context, r *repositoryState, op *GitOperation) {
	if op.WorktreeID == "" || op.Preview.TargetOID == "" {
		op.Status = GitOpUncertain
		op.Summary = "服务重启时分支切换缺少工作树目标，请刷新核对"
		return
	}
	tree, err := s.currentTree(ctx, r, op.WorktreeID)
	if err == nil && tree.Available && tree.Error == "" && tree.Operation == nil &&
		tree.Head != nil && *tree.Head == op.Preview.TargetOID &&
		tree.Changes != nil && tree.Changes.Staged+tree.Changes.Unstaged+tree.Changes.Conflicted == 0 {
		branchMatches := false
		if op.Preview.Detached {
			branchMatches = tree.Detached && tree.Branch == nil
		} else {
			branchMatches = !tree.Detached && tree.Branch != nil && *tree.Branch == op.Preview.Target
		}
		if branchMatches {
			op.Status = GitOpSucceeded
			op.ActualOID = op.Preview.TargetOID
			op.Summary = "服务重启对账确认工作树已切换至目标位置"
			return
		}
	}
	op.Status = GitOpUncertain
	op.Summary = "服务重启时分支切换被中断，请刷新并在终端核对工作树与索引状态"
}
