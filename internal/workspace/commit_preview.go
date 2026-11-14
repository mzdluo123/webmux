package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommitIdentity summarizes the effective Git author, committer, and signing settings.
type CommitIdentity struct {
	AuthorName     string `json:"authorName"`
	AuthorEmail    string `json:"authorEmail"`
	AuthorOverride bool   `json:"authorOverride,omitempty"`
	EnvAuthorName  string `json:"envAuthorName,omitempty"`
	EnvAuthorEmail string `json:"envAuthorEmail,omitempty"`
	CommitterName  string `json:"committerName"`
	CommitterEmail string `json:"committerEmail"`
	SignoffTrailer string `json:"signoffTrailer,omitempty"`
	SignEnabled    bool   `json:"signEnabled"`
	SignFormat     string `json:"signFormat,omitempty"`
}

// CommitPreview represents the complete HEAD (or HEAD^ in Amend mode) -> index commit preview bound to a worktree.
type CommitPreview struct {
	ConfirmationID  string             `json:"confirmationId,omitempty"`
	ExpiresAt       *time.Time         `json:"expiresAt,omitempty"`
	RepositoryID    string             `json:"repositoryId"`
	WorktreeID      string             `json:"worktreeId"`
	WorktreePath    string             `json:"worktreePath"`
	HeadState       string             `json:"headState"` // "branch" | "detached" | "unborn"
	Branch          string             `json:"branch,omitempty"`
	HeadOID         string             `json:"headOid,omitempty"`
	HeadMessage     string             `json:"headMessage,omitempty"`
	HeadParents     []string           `json:"headParents"`
	HeadPush        PushCheck          `json:"headPush"`
	Amend           bool               `json:"amend"`
	AmendPushed     bool               `json:"amendPushed"`
	AuthorOverride  string             `json:"authorOverride,omitempty"`
	Signoff         bool               `json:"signoff"`
	ExpectedTreeOID string             `json:"expectedTreeOid,omitempty"`
	IndexDigest     string             `json:"indexDigest"`
	StagedFiles     []CommitFileChange `json:"stagedFiles"`
	UnstagedCount   int                `json:"unstagedCount"`
	UntrackedCount  int                `json:"untrackedCount"`
	ConflictedCount int                `json:"conflictedCount"`
	Operation       string             `json:"operation,omitempty"`
	Identity        CommitIdentity     `json:"identity"`
	ExecutableHooks []string           `json:"executableHooks"`
	Blockers        []Issue            `json:"blockers"`
	Warnings        []Issue            `json:"warnings"`
}

type commitPreviewState struct {
	commonDir       string
	worktreePath    string
	headState       string
	branch          string
	headOID         string
	headMessage     string
	headParents     []string
	headPush        PushCheck
	amend           bool
	amendPushed     bool
	authorOverride  string
	signoff         bool
	expectedParents []string
	expectedTreeOID string
	indexDigest     string
	stagedFiles     []CommitFileChange
	unstagedCount   int
	untrackedCount  int
	conflictedCount int
	operation       string
	identity        CommitIdentity
	executableHooks []string
	blockers        []Issue
	warnings        []Issue
}

var commitHookNames = []string{"pre-commit", "prepare-commit-msg", "commit-msg", "post-commit"}

// PreviewCommit inspects the full real Git index of the target worktree relative
// to HEAD with default options.
func (s *Service) PreviewCommit(ctx context.Context, repositoryID, worktreeID string) (CommitPreview, error) {
	return s.PreviewCommitWithOptions(ctx, repositoryID, worktreeID, CommitPreviewOptions{})
}

// PreviewCommitWithOptions inspects the full real Git index of the target worktree
// relative to HEAD (or HEAD^ when Amend is enabled) and validates explicit options.
func (s *Service) PreviewCommitWithOptions(ctx context.Context, repositoryID, worktreeID string, opts CommitPreviewOptions) (CommitPreview, error) {
	canonicalAuthor, _, _, err := parseAuthorOverride(opts.AuthorOverride)
	if err != nil {
		return CommitPreview{}, err
	}
	opts.AuthorOverride = canonicalAuthor

	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return CommitPreview{}, err
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return CommitPreview{}, err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return CommitPreview{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return CommitPreview{}, err
	}
	state, err := s.buildCommitPreviewLocked(ctx, r, loc, opts)
	if err != nil {
		return CommitPreview{}, err
	}

	preview := CommitPreview{
		RepositoryID:    repositoryID,
		WorktreeID:      worktreeID,
		WorktreePath:    state.worktreePath,
		HeadState:       state.headState,
		Branch:          state.branch,
		HeadOID:         state.headOID,
		HeadMessage:     state.headMessage,
		HeadParents:     state.headParents,
		HeadPush:        state.headPush,
		Amend:           state.amend,
		AmendPushed:     state.amendPushed,
		AuthorOverride:  state.authorOverride,
		Signoff:         state.signoff,
		ExpectedTreeOID: state.expectedTreeOID,
		IndexDigest:     state.indexDigest,
		StagedFiles:     state.stagedFiles,
		UnstagedCount:   state.unstagedCount,
		UntrackedCount:  state.untrackedCount,
		ConflictedCount: state.conflictedCount,
		Operation:       state.operation,
		Identity:        state.identity,
		ExecutableHooks: state.executableHooks,
		Blockers:        state.blockers,
		Warnings:        state.warnings,
	}
	if len(state.blockers) > 0 {
		return preview, nil
	}
	id, expires, err := s.confirm(confirmation{
		purpose:       purposeCommit,
		repositoryID:  repositoryID,
		worktreeID:    worktreeID,
		commitPreview: &state,
	})
	if err != nil {
		return CommitPreview{}, err
	}
	preview.ConfirmationID = id
	preview.ExpiresAt = &expires
	return preview, nil
}

func (s *Service) buildCommitPreviewLocked(ctx context.Context, r *repositoryState, loc WorktreeLocation, opts CommitPreviewOptions) (commitPreviewState, error) {
	canonicalAuthor, overrideName, overrideEmail, err := parseAuthorOverride(opts.AuthorOverride)
	if err != nil {
		return commitPreviewState{}, err
	}

	branch, detached, err := s.symbolicRefHead(ctx, loc.Path)
	if err != nil {
		return commitPreviewState{}, err
	}
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	headState := "branch"
	switch {
	case detached:
		headState = "detached"
	case headOID == "":
		headState = "unborn"
	}

	indexDigest, expectedTreeOID, hasUnmerged, err := s.readFullIndexTree(ctx, loc.Path)
	if err != nil {
		return commitPreviewState{}, err
	}
	emptyTree, err := s.emptyTreeOID(ctx, loc.Path)
	if err != nil {
		return commitPreviewState{}, err
	}

	var headMeta commitFullMeta
	headParents := []string{}
	headPush := PushCheck{State: PushUnknown, Branches: []string{}}
	if headOID != "" {
		headMeta, err = s.readCommitFullMeta(ctx, loc.Path, headOID)
		if err != nil {
			return commitPreviewState{}, err
		}
		headParents = headMeta.Parents
		if r != nil {
			headPush = s.checkPushed(ctx, r, headOID)
		}
	}

	baseTree := headOID
	if baseTree == "" {
		baseTree = emptyTree
	}
	var expectedParents []string
	if headOID != "" {
		expectedParents = []string{headOID}
	}
	baseTreeOID := emptyTree
	if headOID != "" {
		baseTreeOID = headMeta.TreeOID
	}

	if opts.Amend && headOID != "" {
		expectedParents = append([]string(nil), headMeta.Parents...)
		if len(headMeta.Parents) == 0 {
			baseTree = emptyTree
			baseTreeOID = emptyTree
		} else {
			baseTree = headMeta.Parents[0]
			if parentTree, pErr := s.readTreeOIDForCommit(ctx, loc.Path, headMeta.Parents[0]); pErr == nil {
				baseTreeOID = parentTree
			}
		}
	}

	var stagedFiles []CommitFileChange
	if !hasUnmerged {
		stagedFiles, err = readStagedFileChanges(ctx, loc.Path, baseTree)
		if err != nil {
			return commitPreviewState{}, err
		}
	}
	if stagedFiles == nil {
		stagedFiles = []CommitFileChange{}
	}

	statusData, err := ReadGit(ctx, loc.Path, maxWorkspaceGitOutput, "status", "--porcelain=v2", "-z", "--untracked-files=normal")
	if err != nil {
		return commitPreviewState{}, err
	}
	st, err := parseStatus(statusData)
	if err != nil {
		return commitPreviewState{}, problemf(ErrInternal, "%v", err)
	}

	ident, err := s.readCommitIdentity(ctx, loc.Path)
	if err != nil {
		return commitPreviewState{}, err
	}
	if canonicalAuthor != "" {
		ident.AuthorName = overrideName
		ident.AuthorEmail = overrideEmail
		ident.AuthorOverride = true
	} else if opts.Amend && headOID != "" {
		ident.AuthorName = headMeta.AuthorName
		ident.AuthorEmail = headMeta.AuthorEmail
	}
	if opts.Signoff {
		ident.SignoffTrailer = computeSignoffTrailer(ident)
	}

	hooks := s.detectCommitHooks(ctx, loc.Path)
	var opName string
	if op := detectOperation(loc.Path); op != nil {
		opName = *op
	}

	blockers := make([]Issue, 0)
	warnings := make([]Issue, 0)

	if hasUnmerged || st.changes.Conflicted > 0 {
		blockers = append(blockers, Issue{
			Code:    "conflicted",
			Message: "存在未解决的合并冲突，请先解决冲突后再提交",
		})
	}
	if opName != "" {
		blockers = append(blockers, Issue{
			Code:    "operation_in_progress",
			Message: fmt.Sprintf("工作树正在进行未完成的 %s 操作，请在终端完成或中止后再提交", opName),
		})
	}
	if detached {
		blockers = append(blockers, Issue{
			Code:    "detached_head",
			Message: "当前处于游离 HEAD 状态，请先创建或切换到本地分支后再提交",
		})
	}
	if opts.Amend {
		if headState == "unborn" {
			blockers = append(blockers, Issue{
				Code:    "unborn_amend",
				Message: "当前分支尚无初始提交（unborn HEAD），无法修正上一次提交",
			})
		} else if !hasUnmerged && st.changes.Conflicted == 0 && len(headMeta.Parents) <= 1 && expectedTreeOID == baseTreeOID && headMeta.TreeOID != baseTreeOID {
			blockers = append(blockers, Issue{
				Code:    "empty_amend",
				Message: "修正后的提交树与父提交完全一致（将退化为空提交），请保留至少一项更改或取消修正模式",
			})
		}
	} else if len(stagedFiles) == 0 && !hasUnmerged && st.changes.Conflicted == 0 {
		blockers = append(blockers, Issue{
			Code:    "empty_index",
			Message: "暂存区为空，没有可提交的已暂存修改",
		})
	}
	if strings.TrimSpace(ident.AuthorName) == "" || strings.TrimSpace(ident.AuthorEmail) == "" ||
		strings.TrimSpace(ident.CommitterName) == "" || strings.TrimSpace(ident.CommitterEmail) == "" {
		blockers = append(blockers, Issue{
			Code:    "missing_identity",
			Message: "缺少 Git user.name 或 user.email 身份配置，请先配置后再提交",
		})
	}

	amendPushed := opts.Amend && headPush.State == PushPushed
	if amendPushed {
		warnings = append(warnings, Issue{
			Code:    "amend_pushed",
			Message: "该提交已推送至远端，修正将改写提交历史并导致分支与上游分叉",
		})
	}
	if st.changes.Unstaged > 0 || st.changes.Untracked > 0 {
		warnings = append(warnings, Issue{
			Code:    "remaining_changes",
			Message: fmt.Sprintf("仍有 %d 项未暂存修改与 %d 个未跟踪文件不会进入本次提交", st.changes.Unstaged, st.changes.Untracked),
		})
	}
	if len(hooks) > 0 {
		warnings = append(warnings, Issue{
			Code:    "hooks_active",
			Message: "提交时将运行仓库可执行钩子: " + strings.Join(hooks, ", "),
		})
	}

	return commitPreviewState{
		commonDir:       loc.CommonDir,
		worktreePath:    loc.Path,
		headState:       headState,
		branch:          branch,
		headOID:         headOID,
		headMessage:     headMeta.Message,
		headParents:     headParents,
		headPush:        headPush,
		amend:           opts.Amend,
		amendPushed:     amendPushed,
		authorOverride:  canonicalAuthor,
		signoff:         opts.Signoff,
		expectedParents: expectedParents,
		expectedTreeOID: expectedTreeOID,
		indexDigest:     indexDigest,
		stagedFiles:     stagedFiles,
		unstagedCount:   st.changes.Unstaged,
		untrackedCount:  st.changes.Untracked,
		conflictedCount: st.changes.Conflicted,
		operation:       opName,
		identity:        ident,
		executableHooks: hooks,
		blockers:        blockers,
		warnings:        warnings,
	}, nil
}

func readStagedFileChanges(ctx context.Context, cwd, baseTree string) ([]CommitFileChange, error) {
	numData, err := ReadGit(ctx, cwd, maxHistoryOutput,
		"diff-index", "--cached", "--numstat", "-z", "--no-ext-diff", "--no-textconv", "--no-renames", baseTree, "--",
	)
	if err != nil {
		return nil, err
	}
	binaryPaths := map[string]bool{}
	for _, rec := range bytes.Split(numData, []byte{0}) {
		line := strings.TrimPrefix(string(rec), "\n")
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) == 3 && parts[0] == "-" && parts[1] == "-" {
			binaryPaths[parts[2]] = true
		}
	}

	files := []CommitFileChange{}
	phase := 0
	var current CommitFileChange
	err = ReadGitRecords(ctx, cwd, func(record string) error {
		trimmed := strings.TrimPrefix(record, "\n")
		switch phase {
		case 0:
			fields := strings.Fields(trimmed)
			if len(fields) != 5 || !strings.HasPrefix(fields[0], ":") {
				return problemf(ErrInternal, "无效暂存区文件差异输出")
			}
			oldMode := strings.TrimPrefix(fields[0], ":")
			newMode := fields[1]
			current = CommitFileChange{
				OldMode:     oldMode,
				NewMode:     newMode,
				OldOID:      fields[2],
				NewOID:      fields[3],
				Status:      fields[4],
				ModeChanged: oldMode != "000000" && newMode != "000000" && oldMode != newMode,
			}
			phase = 1
		case 1:
			current.OldPath = record
			current.NewPath = record
			if strings.HasPrefix(current.Status, "R") || strings.HasPrefix(current.Status, "C") {
				phase = 2
			} else {
				current.Binary = binaryPaths[current.NewPath]
				files = append(files, current)
				phase = 0
			}
		case 2:
			current.NewPath = record
			current.Binary = binaryPaths[current.NewPath] || binaryPaths[current.OldPath]
			files = append(files, current)
			phase = 0
		}
		return nil
	}, "diff-index", "--cached", "--raw", "-z", "--no-abbrev", "--no-ext-diff", "--no-textconv", "--find-renames", baseTree, "--")
	if err != nil {
		return nil, err
	}
	if phase != 0 {
		return nil, problemf(ErrInternal, "暂存区文件差异记录不完整")
	}
	return files, nil
}

func (s *Service) symbolicRefHead(ctx context.Context, cwd string) (branch string, detached bool, err error) {
	if s.gitPath == "" {
		return "", false, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	stdout, run := s.runGit(ctx, cwd, []string{"GIT_OPTIONAL_LOCKS=0"}, "symbolic-ref", "-q", "HEAD")
	if run.err == nil {
		ref := strings.TrimSpace(stdout.String())
		return strings.TrimPrefix(ref, "refs/heads/"), false, nil
	}
	if run.ctxErr != nil {
		return "", false, problemf(ErrTimeout, "Git symbolic-ref 超时: %v", run.ctxErr)
	}
	var exit *exec.ExitError
	if errors.As(run.err, &exit) && exit.ExitCode() == 1 {
		return "", true, nil
	}
	return "", false, problemf(ErrInternal, "读取 HEAD 引用失败: %s", strings.TrimSpace(run.stderr))
}
