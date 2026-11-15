package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/worktreefs"
)

// ExecuteDiscard re-verifies every previewed working tree file SHA-256 and index
// identity under the per-worktree gate, records a bounded recovery snapshot OID
// in git-operations.json, and atomically overwrites or removes target paths.
func (s *Service) ExecuteDiscard(ctx context.Context, req ExecuteDiscardRequest) (DiscardResult, error) {
	reqHash := hashOperationRequest(
		GitOpDiscard,
		req.RepositoryID,
		req.WorktreeID,
		req.ConfirmationID,
		fmtBool(req.AcknowledgeSessions),
	)
	return runIdempotentOperation(
		ctx, s, req.RepositoryID, req.ClientRequestID, GitOpDiscard, reqHash,
		func(op GitOperation) (DiscardResult, error) {
			return s.replayDiscardOperation(ctx, req.RepositoryID, req.WorktreeID, op)
		},
		func(opID string) (DiscardResult, error) {
			return s.executeDiscardOnce(ctx, req, opID, reqHash)
		},
	)
}

type verifiedDiscardPlan struct {
	spec          discardTargetSpec
	diff          StagingFileDiff
	nextTextBytes []byte
}

func (s *Service) executeDiscardOnce(ctx context.Context, req ExecuteDiscardRequest, opID, reqHash string) (DiscardResult, error) {
	if req.ConfirmationID == "" {
		return DiscardResult{}, problemf(ErrInvalid, "缺少丢弃修改确认凭证")
	}
	if s.gitPath == "" {
		return DiscardResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	c, ok := s.takeConfirmation(req.ConfirmationID, purposeDiscard, req.RepositoryID, req.WorktreeID)
	if !ok || c.discard == nil {
		return DiscardResult{}, problemf(ErrStale, "丢弃修改预览确认已过期或失效，请重新预览")
	}
	confirmed := *c.discard

	loc, err := s.locateEditableWorktree(ctx, req.RepositoryID, req.WorktreeID)
	if err != nil {
		return DiscardResult{}, err
	}
	r, err := s.repository(req.RepositoryID)
	if err != nil {
		return DiscardResult{}, err
	}

	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return DiscardResult{}, err
	}
	defer release(r)
	if err := s.acquireWorktreeWrite(wCtx, loc.Path); err != nil {
		return DiscardResult{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return DiscardResult{}, err
	}
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return DiscardResult{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	if err := checkWorktreeIndexLock(gitDir); err != nil {
		return DiscardResult{}, err
	}

	currentSessions := s.runningSessions(loc.Path)
	if loc.CommonDir != confirmed.commonDir || loc.Path != confirmed.worktreePath || !sameSessions(confirmed.sessions, currentSessions) {
		return DiscardResult{}, problemf(ErrStale, "工作树状态或活跃会话已变化，请重新预览")
	}
	if len(currentSessions) > 0 && !req.AcknowledgeSessions {
		return DiscardResult{}, problemf(ErrBlocked, "工作树中存在活跃会话，请确认会话影响后再丢弃修改")
	}

	headOID, _ := s.resolveCommitReadOnly(wCtx, loc.Path, "HEAD")
	plans := make([]verifiedDiscardPlan, 0, len(confirmed.targets))
	discardedPaths := make([]string, 0, len(confirmed.targets))

	// Phase 1: Re-verify EVERY target file's SHA-256 and index identity BEFORE
	// writing a single byte on disk so concurrent CLI agent edits are never lost.
	for _, spec := range confirmed.targets {
		diff, dErr := s.inspectFileLocked(wCtx, loc, headOID, StageAreaUnstaged, spec.identity.Path, "", "")
		if dErr != nil {
			return DiscardResult{}, dErr
		}
		if diff.Conflicted {
			return DiscardResult{}, problemf(ErrConflicted, "文件 %s 存在未解决的合并冲突", spec.identity.Path)
		}
		if diff.Identity != spec.identity {
			return DiscardResult{}, problemf(ErrStale, "文件 %s 在预览后已被修改，已拒绝覆盖；请刷新并重新预览", spec.identity.Path)
		}

		plan := verifiedDiscardPlan{spec: spec, diff: diff}
		if spec.action == "partial_text" {
			if !diff.HunkStageAvailable {
				return DiscardResult{}, problemf(ErrConflict, "文件 %s 当前不支持局部丢弃: %s", spec.identity.Path, diff.DegradeReason)
			}
			if len(spec.lineSelections) > 0 {
				selMap, lErr := validateLineSelections(diff.Hunks, spec.lineSelections)
				if lErr != nil {
					return DiscardResult{}, lErr
				}
				plan.nextTextBytes = spliceLineSelections(diff.baseBytes, diff.targetBytes, selMap, false)
			} else {
				discardSet, hErr := validateRequestedHunks(diff.Hunks, spec.hunkIDs)
				if hErr != nil {
					return DiscardResult{}, hErr
				}
				keep := make(map[string]bool, len(diff.Hunks))
				for _, h := range diff.Hunks {
					if !discardSet[h.ID] {
						keep[h.ID] = true
					}
				}
				plan.nextTextBytes = spliceHunkSubset(diff.baseBytes, diff.targetBytes, keep)
			}
		}
		plans = append(plans, plan)
		discardedPaths = append(discardedPaths, spec.identity.Path)
	}

	// Phase 2: Persist pre-discard working tree bytes into a bounded recovery commit
	// under refs/webmux/discard-recovery/<opID> before mutating any file on disk.
	recoveryOID, recoveryRef, recoveryCmd, recErr := s.saveDiscardRecoverySnapshot(wCtx, loc.Path, opID, plans)
	if recErr != nil {
		return DiscardResult{}, recErr
	}

	branch, _, _ := s.symbolicRefHead(wCtx, loc.Path)
	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    req.RepositoryID,
		WorktreeID:      req.WorktreeID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpDiscard,
		Status:          GitOpRunning,
		ConfirmationID:  req.ConfirmationID,
		StartedAt:       time.Now().UTC(),
		Summary:         confirmed.summary,
		RecoveryOID:     recoveryOID,
		RecoveryCommand: recoveryCmd,
		Preview: GitOperationPreview{
			WorktreePath: loc.Path,
			Branch:       branch,
			HeadOID:      headOID,
			Target:       confirmed.scope,
		},
	}); err != nil {
		return DiscardResult{}, err
	}
	if shouldAbortBeforeGit(ctx, opID) {
		return DiscardResult{}, errSimulatedCrash
	}

	// Phase 3: Apply verified file mutations on disk without touching .git/index.
	execCtx, execCancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer execCancel()
	for _, plan := range plans {
		if err := s.applySingleDiscardPlan(execCtx, loc.Path, plan); err != nil {
			s.finalizeOperation(opID, func(op *GitOperation) {
				op.Status = GitOpFailed
				op.FailureKind = classifyFailureKind(err)
				op.Summary = err.Error()
			})
			return DiscardResult{}, err
		}
	}
	if shouldAbortAfterGit(ctx, opID) {
		return DiscardResult{}, errSimulatedCrash
	}

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	afterTree := s.refreshWorktreeLocked(refreshCtx, r, req.WorktreeID)

	summary := confirmed.summary
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = recoveryOID
		op.RecoveryOID = recoveryOID
		op.RecoveryCommand = recoveryCmd
		op.Summary = summary
	})

	return DiscardResult{
		OperationID:     opID,
		Scope:           confirmed.scope,
		Summary:         summary,
		RecoveryOID:     recoveryOID,
		RecoveryRef:     recoveryRef,
		RecoveryCommand: recoveryCmd,
		DiscardedPaths:  discardedPaths,
		Worktree:        afterTree,
	}, nil
}

func (s *Service) applySingleDiscardPlan(ctx context.Context, rootPath string, plan verifiedDiscardPlan) error {
	id := plan.spec.identity
	cleanPath := id.Path

	switch plan.spec.action {
	case "delete_untracked":
		if err := worktreefs.RemoveVerified(rootPath, cleanPath, id.TargetSHA256, maxWorkspaceGitOutput); err != nil {
			if errors.Is(err, worktreefs.ErrStale) {
				return problemf(ErrStale, "未跟踪文件 %s 在写入前已被外部修改，已拒绝删除", cleanPath)
			}
			return problemf(ErrConflict, "删除未跟踪文件 %s 失败: %v", cleanPath, err)
		}
		return nil

	case "partial_text":
		_, err := worktreefs.WriteRegular(
			rootPath, cleanPath, plan.nextTextBytes,
			id.TargetMode == "100755", true, id.TargetSHA256, maxWorkspaceGitOutput,
		)
		if err != nil {
			if errors.Is(err, worktreefs.ErrStale) {
				return problemf(ErrStale, "文件 %s 在写入前已被外部修改，已拒绝覆盖", cleanPath)
			}
			return problemf(ErrConflict, "写入文件 %s 失败: %v", cleanPath, err)
		}
		return nil

	default:
		// Whole-file restore/overwrite from .git/index entry (id.BaseOID, id.BaseMode).
		if isRegularGitMode(id.BaseMode) && !s.hasTransformAttributes(ctx, rootPath, cleanPath) {
			_, err := worktreefs.WriteRegular(
				rootPath, cleanPath, plan.diff.baseBytes,
				id.BaseMode == "100755", true, id.TargetSHA256, maxWorkspaceGitOutput,
			)
			if err != nil {
				if errors.Is(err, worktreefs.ErrStale) {
					return problemf(ErrStale, "文件 %s 在写入前已被外部修改，已拒绝覆盖", cleanPath)
				}
				return problemf(ErrConflict, "恢复文件 %s 失败: %v", cleanPath, err)
			}
			return nil
		}
		if id.TargetMode != "" {
			if err := worktreefs.RemoveVerified(rootPath, cleanPath, id.TargetSHA256, maxWorkspaceGitOutput); err != nil {
				if errors.Is(err, worktreefs.ErrStale) {
					return problemf(ErrStale, "文件 %s 在写入前已被外部修改，已拒绝覆盖", cleanPath)
				}
				return problemf(ErrConflict, "清理文件 %s 失败: %v", cleanPath, err)
			}
		}
		if _, _, err := s.gitInputOutput(ctx, rootPath, nil, nil, "checkout-index", "--force", "--", cleanPath); err != nil {
			return err
		}
		return nil
	}
}

func (s *Service) saveDiscardRecoverySnapshot(
	ctx context.Context,
	cwd, opID string,
	plans []verifiedDiscardPlan,
) (commitOID, refName, recoveryCmd string, err error) {
	entries := make([]discardTreeEntry, 0, len(plans))
	paths := make([]string, 0, len(plans))
	for _, plan := range plans {
		id := plan.spec.identity
		if id.TargetMode == "" || !isBackupableMode(id.TargetMode) {
			continue
		}
		out, _, hErr := s.gitInputOutput(ctx, cwd, bytes.NewReader(plan.diff.worktreeRaw), nil, "hash-object", "-w", "--no-filters", "--stdin")
		if hErr != nil {
			return "", "", "", hErr
		}
		blobOID := strings.TrimSpace(string(out))
		if !isObjectID(blobOID) {
			return "", "", "", problemf(ErrInternal, "保存丢弃前恢复快照失败")
		}
		entries = append(entries, discardTreeEntry{mode: id.TargetMode, oid: blobOID, path: id.Path})
		paths = append(paths, id.Path)
	}
	if len(entries) == 0 {
		return "", "", "", nil
	}

	treeOID, err := s.buildRecoveryTree(ctx, cwd, entries)
	if err != nil {
		return "", "", "", err
	}

	msg := fmt.Sprintf("webmux-discard-recovery:%s\n", opID)
	commitEnv := []string{
		"GIT_AUTHOR_NAME=WebMux Recovery",
		"GIT_AUTHOR_EMAIL=recovery@webmux.local",
		"GIT_COMMITTER_NAME=WebMux Recovery",
		"GIT_COMMITTER_EMAIL=recovery@webmux.local",
	}
	commitOut, _, err := s.gitInputOutput(ctx, cwd, strings.NewReader(msg), commitEnv, "commit-tree", treeOID)
	if err != nil {
		return "", "", "", err
	}
	commitOID = strings.TrimSpace(string(commitOut))
	if !isObjectID(commitOID) {
		return "", "", "", problemf(ErrInternal, "创建恢复快照提交失败")
	}

	refName = discardRecoveryPrefix + opID
	if _, _, err := s.gitInputOutput(ctx, cwd, nil, nil, "update-ref", "-m", "webmux discard recovery", refName, commitOID); err != nil {
		return "", "", "", err
	}
	s.pruneDiscardRecoveryRefsBestEffort(ctx, cwd)
	return commitOID, refName, formatDiscardRecoveryCommand(commitOID, paths), nil
}

type discardTreeEntry struct {
	mode string
	oid  string
	path string
}

type discardTreeDir struct {
	blobs map[string]discardTreeEntry
	dirs  map[string]*discardTreeDir
}

func (s *Service) buildRecoveryTree(ctx context.Context, cwd string, entries []discardTreeEntry) (string, error) {
	root := &discardTreeDir{
		blobs: make(map[string]discardTreeEntry),
		dirs:  make(map[string]*discardTreeDir),
	}
	for _, e := range entries {
		parts := strings.Split(e.path, "/")
		cur := root
		for _, seg := range parts[:len(parts)-1] {
			next, ok := cur.dirs[seg]
			if !ok {
				next = &discardTreeDir{
					blobs: make(map[string]discardTreeEntry),
					dirs:  make(map[string]*discardTreeDir),
				}
				cur.dirs[seg] = next
			}
			cur = next
		}
		cur.blobs[parts[len(parts)-1]] = e
	}
	return s.writeDiscardTreeDir(ctx, cwd, root)
}

func (s *Service) writeDiscardTreeDir(ctx context.Context, cwd string, node *discardTreeDir) (string, error) {
	var buf bytes.Buffer
	dirNames := make([]string, 0, len(node.dirs))
	for name := range node.dirs {
		dirNames = append(dirNames, name)
	}
	slices.Sort(dirNames)
	for _, name := range dirNames {
		subOID, err := s.writeDiscardTreeDir(ctx, cwd, node.dirs[name])
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&buf, "040000 tree %s\t%s\x00", subOID, name)
	}
	blobNames := make([]string, 0, len(node.blobs))
	for name := range node.blobs {
		blobNames = append(blobNames, name)
	}
	slices.Sort(blobNames)
	for _, name := range blobNames {
		b := node.blobs[name]
		fmt.Fprintf(&buf, "%s blob %s\t%s\x00", b.mode, b.oid, name)
	}
	out, _, err := s.gitInputOutput(ctx, cwd, bytes.NewReader(buf.Bytes()), nil, "mktree", "-z")
	if err != nil {
		return "", err
	}
	oid := strings.TrimSpace(string(out))
	if !isObjectID(oid) {
		return "", problemf(ErrInternal, "写入恢复快照树对象失败")
	}
	return oid, nil
}

func isBackupableMode(mode string) bool {
	return mode == "100644" || mode == "100755" || mode == "120000"
}

func formatDiscardRecoveryCommand(commitOID string, paths []string) string {
	if commitOID == "" {
		return ""
	}
	if len(paths) > 0 && len(paths) <= 4 {
		quoted := make([]string, len(paths))
		for i, p := range paths {
			quoted[i] = shellQuoteArg(p)
		}
		return fmt.Sprintf("git restore --source=%s --worktree -- %s", commitOID, strings.Join(quoted, " "))
	}
	return fmt.Sprintf("git restore --source=%s --worktree -- .", commitOID)
}

func shellQuoteArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`!&*()[]{};|<>?") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (s *Service) pruneDiscardRecoveryRefsBestEffort(ctx context.Context, cwd string) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "for-each-ref", "--sort=-creatordate", "--format=%(refname)", discardRecoveryPrefix)
	if err != nil || len(data) == 0 {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) <= maxDiscardRecoveryRefs {
		return
	}
	for _, oldRef := range lines[maxDiscardRecoveryRefs:] {
		oldRef = strings.TrimSpace(oldRef)
		if strings.HasPrefix(oldRef, discardRecoveryPrefix) {
			_, _ = s.git(ctx, cwd, "update-ref", "-d", oldRef)
		}
	}
}
