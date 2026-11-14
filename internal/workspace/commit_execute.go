package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const (
	commitExecuteTimeout     = 2 * time.Minute
	CommitOutcomeMatched     = "matched"
	CommitOutcomeNeedsReview = "needs_review"
	CommitOutcomeUncertain   = "uncertain"
)

// ExecuteCommitRequest confirms and runs a previously previewed full-index commit.
type ExecuteCommitRequest struct {
	RepositoryID           string `json:"repositoryId"`
	WorktreeID             string `json:"worktreeId"`
	ConfirmationID         string `json:"confirmationId"`
	ClientRequestID        string `json:"clientRequestId,omitempty"`
	Message                string `json:"message"`
	AcknowledgePushedAmend bool   `json:"acknowledgePushedAmend,omitempty"`
}

// CommitResult classifies the outcome of ExecuteCommit after reflog attribution
// and {oid, parents, tree, author, committer} verification.
type CommitResult struct {
	OperationID     string   `json:"operationId,omitempty"`
	Outcome         string   `json:"outcome"` // "matched" | "needs_review" | "uncertain"
	Amended         bool     `json:"amended,omitempty"`
	CommitOID       string   `json:"commitOid,omitempty"`
	Parents         []string `json:"parents,omitempty"`
	TreeOID         string   `json:"treeOid,omitempty"`
	ExpectedTreeOID string   `json:"expectedTreeOid"`
	Subject         string   `json:"subject,omitempty"`
	AuthorName      string   `json:"authorName,omitempty"`
	AuthorEmail     string   `json:"authorEmail,omitempty"`
	CommitterName   string   `json:"committerName,omitempty"`
	CommitterEmail  string   `json:"committerEmail,omitempty"`
	ReviewReasons   []string `json:"reviewReasons,omitempty"`
}

type afterCommitHookKey struct{}

// ExecuteCommit verifies the preview confirmation under the target worktree write
// gate, runs normal `git commit` respecting hooks/filters/signing, never removes
// external Git lock files, and reconciles the created commit via GIT_REFLOG_ACTION.
func (s *Service) ExecuteCommit(ctx context.Context, req ExecuteCommitRequest) (CommitResult, error) {
	reqHash := hashOperationRequest(GitOpCommit, req.RepositoryID, req.WorktreeID, req.ConfirmationID, req.Message, fmtBool(req.AcknowledgePushedAmend))
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.ClientRequestID, GitOpCommit, reqHash,
		replayCommitOperation,
		func(opID string) (CommitResult, error) {
			return s.executeCommitOnce(ctx, req, opID, reqHash)
		},
	)
}

func (s *Service) executeCommitOnce(ctx context.Context, req ExecuteCommitRequest, opID, reqHash string) (CommitResult, error) {
	if strings.TrimSpace(req.Message) == "" {
		return CommitResult{}, problemf(ErrInvalid, "提交说明不能为空")
	}
	if req.ConfirmationID == "" {
		return CommitResult{}, problemf(ErrInvalid, "缺少提交预览确认凭证")
	}
	if s.gitPath == "" {
		return CommitResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}

	c, ok := s.takeConfirmation(req.ConfirmationID, purposeCommit, req.RepositoryID, req.WorktreeID)
	if !ok || c.commitPreview == nil {
		return CommitResult{}, problemf(ErrStale, "提交预览已过期或失效，保留当前提交说明草稿，请重新预览")
	}
	confirmed := *c.commitPreview

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return CommitResult{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return CommitResult{}, err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return CommitResult{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return CommitResult{}, err
	}
	if loc.CommonDir != confirmed.commonDir || loc.Path != confirmed.worktreePath {
		return CommitResult{}, problemf(ErrStale, "工作树路径或仓库身份已变化，请重新预览")
	}

	recheckCtx, recheckCancel := context.WithTimeout(ctx, readTimeout)
	current, err := s.buildCommitPreviewLocked(recheckCtx, r, loc, CommitPreviewOptions{
		Amend:          confirmed.amend,
		AuthorOverride: confirmed.authorOverride,
		Signoff:        confirmed.signoff,
	})
	recheckCancel()
	if err != nil {
		return CommitResult{}, err
	}
	if len(current.blockers) > 0 {
		return CommitResult{}, problemf(ErrBlocked, "提交被阻止：%s；保留草稿，请重新预览", issueMessages(current.blockers))
	}
	if current.headState != confirmed.headState ||
		current.branch != confirmed.branch ||
		current.headOID != confirmed.headOID ||
		current.indexDigest != confirmed.indexDigest ||
		current.expectedTreeOID != confirmed.expectedTreeOID {
		return CommitResult{}, problemf(ErrStale, "预览后 HEAD 或暂存区已被外部修改，当前确认已失效；保留提交说明草稿，请重新预览")
	}
	if confirmed.amend && (confirmed.amendPushed || current.amendPushed) && !req.AcknowledgePushedAmend {
		return CommitResult{}, problemf(ErrBlocked, "该提交已推送至远端，修正将改写提交历史并导致分支与上游分叉；请显式勾选风险确认后再执行")
	}

	startSummary := "正在创建提交"
	if confirmed.amend {
		startSummary = "正在修正（Amend）上一次提交"
	}
	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    req.RepositoryID,
		WorktreeID:      req.WorktreeID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpCommit,
		Status:          GitOpRunning,
		ConfirmationID:  req.ConfirmationID,
		StartedAt:       time.Now().UTC(),
		Summary:         startSummary,
		Preview: GitOperationPreview{
			WorktreePath:    loc.Path,
			HeadState:       confirmed.headState,
			Branch:          confirmed.branch,
			HeadOID:         confirmed.headOID,
			ExpectedTreeOID: confirmed.expectedTreeOID,
			ExpectedParents: append([]string(nil), confirmed.expectedParents...),
			Amend:           confirmed.amend,
			AuthorOverride:  confirmed.authorOverride,
			Signoff:         confirmed.signoff,
		},
	}); err != nil {
		return CommitResult{}, err
	}
	if shouldAbortBeforeGit(ctx, opID) {
		return CommitResult{}, errSimulatedCrash
	}

	reflogTag := "webmux-commit:" + opID
	gitArgs := []string{
		"-C", loc.Path,
		"-c", "core.logAllRefUpdates=always",
		"-c", "user.useConfigOnly=true",
		"commit", "-F", "-",
	}
	if confirmed.amend {
		gitArgs = append(gitArgs, "--amend")
	}
	if confirmed.signoff {
		gitArgs = append(gitArgs, "--signoff")
	}
	if confirmed.authorOverride != "" {
		gitArgs = append(gitArgs, "--author="+confirmed.authorOverride)
	}

	commitCtx, commitCancel := context.WithTimeout(context.WithoutCancel(ctx), commitExecuteTimeout)
	stdout := &boundedOutput{limit: maxWorkspaceGitOutput, cancel: commitCancel}
	run := runGitProcess(commitCtx, gitProcess{
		path:        s.gitPath,
		args:        gitArgs,
		env:         append(append([]string{}, gitEnv...), "GIT_REFLOG_ACTION="+reflogTag),
		stdin:       strings.NewReader(req.Message),
		stdout:      stdout,
		stderrLimit: maxWorkspaceGitError,
	})
	commitCancel()

	if hook, ok := ctx.Value(afterCommitHookKey{}).(func(string)); ok && hook != nil {
		hook(loc.Path)
	}
	if shouldAbortAfterGit(ctx, opID) {
		return CommitResult{}, errSimulatedCrash
	}

	combinedOut := strings.TrimSpace(stdout.String() + "\n" + run.stderr)
	failOp := func(retErr error) (CommitResult, error) {
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = combinedOut
		})
		return CommitResult{}, retErr
	}

	if run.err != nil {
		if isGitLockBusy(run.stderr) {
			return failOp(problemf(ErrLocked, "仓库正被其他进程占用（Git 锁冲突），未删除任何锁文件，请稍后重试"))
		}
		var launch *exec.Error
		if errors.As(run.err, &launch) || errors.Is(run.err, os.ErrNotExist) || errors.Is(run.err, os.ErrPermission) {
			return failOp(problemf(ErrUnavailable, "Git 无法执行: %v", run.err))
		}
	}

	scanCtx, scanCancel := context.WithTimeout(context.Background(), readTimeout)
	defer scanCancel()

	matches, reflogErr := s.scanCommitReflogWithBranch(scanCtx, loc.Path, true, loc.Path, confirmed.branch, reflogTag)
	if run.err != nil && len(matches) == 0 {
		if run.ctxErr != nil {
			latestHead, hErr := s.resolveCommitReadOnly(scanCtx, loc.Path, "HEAD")
			if reflogErr != nil || hErr != nil || latestHead != confirmed.headOID {
				res := CommitResult{
					OperationID:     opID,
					Outcome:         CommitOutcomeUncertain,
					Amended:         confirmed.amend,
					ExpectedTreeOID: confirmed.expectedTreeOID,
					ReviewReasons:   []string{"提交命令超时，无法唯一确认是否已产生提交，请检查提交历史"},
				}
				s.finalizeOperation(opID, func(op *GitOperation) {
					op.Status = GitOpUncertain
					op.Summary = res.ReviewReasons[0]
					op.ReviewReasons = res.ReviewReasons
					op.SanitizedOutput = combinedOut
				})
				return res, nil
			}
			return failOp(problemf(ErrTimeout, "Git commit 执行超时: %v", run.ctxErr))
		}
		return failOp(problemf(ErrHookFailed, "Git commit 执行失败: %s", strings.TrimSpace(run.stderr)))
	}

	if reflogErr != nil || len(matches) == 0 {
		res := CommitResult{
			OperationID:     opID,
			Outcome:         CommitOutcomeUncertain,
			Amended:         confirmed.amend,
			ExpectedTreeOID: confirmed.expectedTreeOID,
			ReviewReasons:   []string{"无法从 HEAD reflog 唯一确认本次操作的提交记录，请刷新历史核对"},
		}
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpUncertain
			op.Summary = res.ReviewReasons[0]
			op.ReviewReasons = res.ReviewReasons
			op.SanitizedOutput = combinedOut
		})
		return res, nil
	}

	actualOID := matches[0]
	meta, metaErr := s.readCommitFullMeta(scanCtx, loc.Path, actualOID)
	if metaErr != nil {
		res := CommitResult{
			OperationID:     opID,
			Outcome:         CommitOutcomeUncertain,
			Amended:         confirmed.amend,
			CommitOID:       actualOID,
			ExpectedTreeOID: confirmed.expectedTreeOID,
			ReviewReasons:   []string{"已定位提交对象但读取提交元数据失败，请刷新历史核对"},
		}
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpUncertain
			op.ActualOID = actualOID
			op.Summary = res.ReviewReasons[0]
			op.ReviewReasons = res.ReviewReasons
			op.SanitizedOutput = combinedOut
		})
		return res, nil
	}

	expectedParents := confirmed.expectedParents
	if !confirmed.amend && expectedParents == nil && confirmed.headOID != "" {
		expectedParents = []string{confirmed.headOID}
	}

	reasons := make([]string, 0)
	if run.err != nil {
		reasons = append(reasons, "提交已写入，但后续钩子或进程返回了非零状态")
	}
	if len(matches) > 1 {
		reasons = append(reasons, "同次提交过程中检测到多次引用更新（例如 post-commit 修正了提交）")
	}
	if meta.TreeOID != confirmed.expectedTreeOID {
		reasons = append(reasons, "实际生成的提交树摘要与预览不一致（可能由 pre-commit 等钩子修改并重新暂存了文件）")
	}
	if !slices.Equal(meta.Parents, expectedParents) {
		reasons = append(reasons, "实际提交的父节点列表与预览不一致")
	}
	if confirmed.identity.AuthorName != "" && (meta.AuthorName != confirmed.identity.AuthorName || meta.AuthorEmail != confirmed.identity.AuthorEmail) {
		reasons = append(reasons, "实际提交作者信息与预览不一致（可能被提交钩子改写）")
	}
	if confirmed.identity.CommitterName != "" && (meta.CommitterName != confirmed.identity.CommitterName || meta.CommitterEmail != confirmed.identity.CommitterEmail) {
		reasons = append(reasons, "实际提交者（Committer）信息与预览不一致")
	}

	latestHead, _ := s.resolveCommitReadOnly(scanCtx, loc.Path, "HEAD")
	if latestHead != "" && latestHead != actualOID {
		if !s.isAncestorReadOnly(scanCtx, loc.Path, actualOID, latestHead) {
			reasons = append(reasons, "提交完成后当前分支 HEAD 被后续改写（如 post-commit --amend）")
		}
	}

	outcome := CommitOutcomeMatched
	ledgerStatus := GitOpSucceeded
	verb := "已创建提交 "
	if confirmed.amend {
		verb = "已修正提交 "
	}
	summary := verb + shortOID(actualOID)
	if len(reasons) > 0 {
		outcome = CommitOutcomeNeedsReview
		ledgerStatus = GitOpNeedsReview
		summary = verb + shortOID(actualOID) + "（需复核钩子或树差异）"
	}
	res := CommitResult{
		OperationID:     opID,
		Outcome:         outcome,
		Amended:         confirmed.amend,
		CommitOID:       actualOID,
		Parents:         meta.Parents,
		TreeOID:         meta.TreeOID,
		ExpectedTreeOID: confirmed.expectedTreeOID,
		Subject:         meta.Subject,
		AuthorName:      meta.AuthorName,
		AuthorEmail:     meta.AuthorEmail,
		CommitterName:   meta.CommitterName,
		CommitterEmail:  meta.CommitterEmail,
		ReviewReasons:   reasons,
	}
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = ledgerStatus
		op.Summary = summary
		op.ActualOID = actualOID
		op.ActualTreeOID = meta.TreeOID
		op.Parents = meta.Parents
		op.Subject = meta.Subject
		op.AuthorName = meta.AuthorName
		op.AuthorEmail = meta.AuthorEmail
		op.CommitterName = meta.CommitterName
		op.CommitterEmail = meta.CommitterEmail
		op.ReviewReasons = reasons
		op.SanitizedOutput = combinedOut
	})
	s.clearWorktreeCommitDraftBestEffort(req.RepositoryID, req.WorktreeID)
	s.readRepository(scanCtx, r)
	return res, nil
}

func shortOID(oid string) string {
	if len(oid) > 8 {
		return oid[:8]
	}
	return oid
}
