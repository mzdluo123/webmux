package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CommitIdentity summarizes the effective Git author, committer, and signing settings.
type CommitIdentity struct {
	AuthorName     string `json:"authorName"`
	AuthorEmail    string `json:"authorEmail"`
	CommitterName  string `json:"committerName"`
	CommitterEmail string `json:"committerEmail"`
	SignEnabled    bool   `json:"signEnabled"`
	SignFormat     string `json:"signFormat,omitempty"`
}

// CommitPreview represents the complete HEAD -> index commit preview bound to a worktree.
type CommitPreview struct {
	ConfirmationID  string             `json:"confirmationId,omitempty"`
	ExpiresAt       *time.Time         `json:"expiresAt,omitempty"`
	RepositoryID    string             `json:"repositoryId"`
	WorktreeID      string             `json:"worktreeId"`
	WorktreePath    string             `json:"worktreePath"`
	HeadState       string             `json:"headState"` // "branch" | "detached" | "unborn"
	Branch          string             `json:"branch,omitempty"`
	HeadOID         string             `json:"headOid,omitempty"`
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
// to HEAD (including any changes previously staged via CLI) and checks blockers.
func (s *Service) PreviewCommit(ctx context.Context, repositoryID, worktreeID string) (CommitPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
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
	state, err := s.buildCommitPreviewLocked(ctx, loc)
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

func (s *Service) buildCommitPreviewLocked(ctx context.Context, loc WorktreeLocation) (commitPreviewState, error) {
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

	baseTree := headOID
	if baseTree == "" {
		baseTree, err = s.emptyTreeOID(ctx, loc.Path)
		if err != nil {
			return commitPreviewState{}, err
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
	if len(stagedFiles) == 0 && !hasUnmerged && st.changes.Conflicted == 0 {
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

func (s *Service) readCommitIdentity(ctx context.Context, cwd string) (CommitIdentity, error) {
	userName, _, err := s.configValue(ctx, cwd, "user.name")
	if err != nil {
		return CommitIdentity{}, err
	}
	userEmail, _, err := s.configValue(ctx, cwd, "user.email")
	if err != nil {
		return CommitIdentity{}, err
	}
	authorNameConf, _, _ := s.configValue(ctx, cwd, "author.name")
	authorEmailConf, _, _ := s.configValue(ctx, cwd, "author.email")
	committerNameConf, _, _ := s.configValue(ctx, cwd, "committer.name")
	committerEmailConf, _, _ := s.configValue(ctx, cwd, "committer.email")
	gpgSign, _, _ := s.configValue(ctx, cwd, "commit.gpgsign")
	gpgFormat, _, _ := s.configValue(ctx, cwd, "gpg.format")

	pick := func(values ...string) string {
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				return v
			}
		}
		return ""
	}
	signEnabled := strings.EqualFold(gpgSign, "true") || gpgSign == "1" || strings.EqualFold(gpgSign, "yes")
	return CommitIdentity{
		AuthorName:     pick(os.Getenv("GIT_AUTHOR_NAME"), authorNameConf, userName),
		AuthorEmail:    pick(os.Getenv("GIT_AUTHOR_EMAIL"), authorEmailConf, userEmail, os.Getenv("EMAIL")),
		CommitterName:  pick(os.Getenv("GIT_COMMITTER_NAME"), committerNameConf, userName),
		CommitterEmail: pick(os.Getenv("GIT_COMMITTER_EMAIL"), committerEmailConf, userEmail, os.Getenv("EMAIL")),
		SignEnabled:    signEnabled,
		SignFormat:     gpgFormat,
	}, nil
}

func (s *Service) detectCommitHooks(ctx context.Context, cwd string) []string {
	active := make([]string, 0, len(commitHookNames))
	for _, name := range commitHookNames {
		data, err := ReadGit(ctx, cwd, 1024, "rev-parse", "--git-path", "hooks/"+name)
		if err != nil {
			continue
		}
		p := outputLine(data)
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			active = append(active, name)
		}
	}
	return active
}
