package workspace

import (
	"bytes"
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
	RepositoryID   string `json:"repositoryId"`
	WorktreeID     string `json:"worktreeId"`
	ConfirmationID string `json:"confirmationId"`
	Message        string `json:"message"`
}

// CommitResult classifies the outcome of ExecuteCommit after reflog attribution
// and {oid, parents, tree} verification.
type CommitResult struct {
	Outcome         string   `json:"outcome"` // "matched" | "needs_review" | "uncertain"
	CommitOID       string   `json:"commitOid,omitempty"`
	Parents         []string `json:"parents,omitempty"`
	TreeOID         string   `json:"treeOid,omitempty"`
	ExpectedTreeOID string   `json:"expectedTreeOid"`
	Subject         string   `json:"subject,omitempty"`
	ReviewReasons   []string `json:"reviewReasons,omitempty"`
}

type afterCommitHookKey struct{}

// ExecuteCommit verifies the preview confirmation under the target worktree write
// gate, runs normal `git commit` respecting hooks/filters/signing, never removes
// external Git lock files, and reconciles the created commit via GIT_REFLOG_ACTION.
func (s *Service) ExecuteCommit(ctx context.Context, req ExecuteCommitRequest) (CommitResult, error) {
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
	current, err := s.buildCommitPreviewLocked(recheckCtx, loc)
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

	opID, err := randomID()
	if err != nil {
		return CommitResult{}, err
	}
	reflogTag := "webmux-commit:" + opID

	commitCtx, commitCancel := context.WithTimeout(context.WithoutCancel(ctx), commitExecuteTimeout)
	stdout := &boundedOutput{limit: maxWorkspaceGitOutput, cancel: commitCancel}
	run := runGitProcess(commitCtx, gitProcess{
		path: s.gitPath,
		args: []string{
			"-C", loc.Path,
			"-c", "core.logAllRefUpdates=always",
			"-c", "user.useConfigOnly=true",
			"commit", "-F", "-",
		},
		env:         append(append([]string{}, gitEnv...), "GIT_REFLOG_ACTION="+reflogTag),
		stdin:       strings.NewReader(req.Message),
		stdout:      stdout,
		stderrLimit: maxWorkspaceGitError,
	})
	commitCancel()

	if hook, ok := ctx.Value(afterCommitHookKey{}).(func(string)); ok && hook != nil {
		hook(loc.Path)
	}

	if run.err != nil {
		if isGitLockBusy(run.stderr) {
			return CommitResult{}, problemf(ErrLocked, "仓库正被其他进程占用（Git 锁冲突），未删除任何锁文件，请稍后重试")
		}
		var launch *exec.Error
		if errors.As(run.err, &launch) || errors.Is(run.err, os.ErrNotExist) || errors.Is(run.err, os.ErrPermission) {
			return CommitResult{}, problemf(ErrUnavailable, "Git 无法执行: %v", run.err)
		}
	}

	scanCtx, scanCancel := context.WithTimeout(context.Background(), readTimeout)
	defer scanCancel()

	matches, reflogErr := s.scanCommitReflog(scanCtx, loc.Path, reflogTag)
	if run.err != nil && len(matches) == 0 {
		if run.ctxErr != nil {
			latestHead, hErr := s.resolveCommitReadOnly(scanCtx, loc.Path, "HEAD")
			if reflogErr != nil || hErr != nil || latestHead != confirmed.headOID {
				return CommitResult{
					Outcome:         CommitOutcomeUncertain,
					ExpectedTreeOID: confirmed.expectedTreeOID,
					ReviewReasons:   []string{"提交命令超时，无法唯一确认是否已产生提交，请检查提交历史"},
				}, nil
			}
			return CommitResult{}, problemf(ErrTimeout, "Git commit 执行超时: %v", run.ctxErr)
		}
		return CommitResult{}, problemf(ErrHookFailed, "Git commit 执行失败: %s", strings.TrimSpace(run.stderr))
	}

	if reflogErr != nil || len(matches) == 0 {
		return CommitResult{
			Outcome:         CommitOutcomeUncertain,
			ExpectedTreeOID: confirmed.expectedTreeOID,
			ReviewReasons:   []string{"无法从 HEAD reflog 唯一确认本次操作的提交记录，请刷新历史核对"},
		}, nil
	}

	actualOID := matches[0]
	actualTree, actualParents, subject, metaErr := s.readCommitTreeAndParents(scanCtx, loc.Path, actualOID)
	if metaErr != nil {
		return CommitResult{
			Outcome:         CommitOutcomeUncertain,
			CommitOID:       actualOID,
			ExpectedTreeOID: confirmed.expectedTreeOID,
			ReviewReasons:   []string{"已定位提交对象但读取提交元数据失败，请刷新历史核对"},
		}, nil
	}

	var expectedParents []string
	if confirmed.headOID != "" {
		expectedParents = []string{confirmed.headOID}
	}

	reasons := make([]string, 0)
	if run.err != nil {
		reasons = append(reasons, "提交已写入，但后续钩子或进程返回了非零状态")
	}
	if len(matches) > 1 {
		reasons = append(reasons, "同次提交过程中检测到多次引用更新（例如 post-commit 修正了提交）")
	}
	if actualTree != confirmed.expectedTreeOID {
		reasons = append(reasons, "实际生成的提交树摘要与预览不一致（可能由 pre-commit 等钩子修改并重新暂存了文件）")
	}
	if !slices.Equal(actualParents, expectedParents) {
		reasons = append(reasons, "实际提交的父节点列表与预览不一致")
	}

	latestHead, _ := s.resolveCommitReadOnly(scanCtx, loc.Path, "HEAD")
	if latestHead != "" && latestHead != actualOID {
		if !s.isAncestorReadOnly(scanCtx, loc.Path, actualOID, latestHead) {
			reasons = append(reasons, "提交完成后当前分支 HEAD 被后续改写（如 post-commit --amend）")
		}
	}

	outcome := CommitOutcomeMatched
	if len(reasons) > 0 {
		outcome = CommitOutcomeNeedsReview
	}
	return CommitResult{
		Outcome:         outcome,
		CommitOID:       actualOID,
		Parents:         actualParents,
		TreeOID:         actualTree,
		ExpectedTreeOID: confirmed.expectedTreeOID,
		Subject:         subject,
		ReviewReasons:   reasons,
	}, nil
}

func (s *Service) scanCommitReflog(ctx context.Context, cwd, reflogTag string) ([]string, error) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "log", "-g", "-n", "64", "--no-show-signature", "--format=%H%x00%gs%x00", "HEAD", "--")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	fields := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	matches := make([]string, 0, 2)
	for i := 0; i+1 < len(fields); i += 2 {
		oid := strings.TrimLeft(string(fields[i]), "\r\n")
		gs := string(fields[i+1])
		if !isObjectID(oid) {
			continue
		}
		if gs == reflogTag || strings.HasPrefix(gs, reflogTag+":") || strings.HasPrefix(gs, reflogTag+" ") {
			matches = append(matches, oid)
		}
	}
	return matches, nil
}

func (s *Service) readCommitTreeAndParents(ctx context.Context, cwd, oid string) (treeOID string, parents []string, subject string, err error) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "log", "-n", "1", "--no-show-signature", "--format=%T%x00%P%x00%s%x00", oid, "--")
	if err != nil {
		return "", nil, "", err
	}
	fields := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	if len(fields) < 3 {
		return "", nil, "", problemf(ErrInternal, "提交元数据格式无效")
	}
	treeOID = strings.TrimSpace(string(fields[0]))
	rawParents := strings.TrimSpace(string(fields[1]))
	if rawParents != "" {
		parents = strings.Fields(rawParents)
	}
	subject = string(fields[2])
	return treeOID, parents, subject, nil
}

func (s *Service) isAncestorReadOnly(ctx context.Context, cwd, ancestor, descendant string) bool {
	if s.gitPath == "" {
		return false
	}
	_, run := s.runGit(ctx, cwd, []string{"GIT_OPTIONAL_LOCKS=0"}, "merge-base", "--is-ancestor", ancestor, descendant)
	return run.err == nil
}
