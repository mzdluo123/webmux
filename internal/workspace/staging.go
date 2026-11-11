package workspace

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
)

type StageArea string

const (
	StageAreaUnstaged   StageArea = "unstaged"
	StageAreaStaged     StageArea = "staged"
	StageAreaConflicted StageArea = "conflicted"
)

// StageFileIdentity binds a stage or unstage request to the exact server-observed
// content and file modes on both sides of the comparison.
type StageFileIdentity struct {
	Area         StageArea `json:"area"`
	Path         string    `json:"path"`
	OldPath      string    `json:"oldPath,omitempty"`
	OldBaseOID   string    `json:"oldBaseOid,omitempty"`
	OldBaseMode  string    `json:"oldBaseMode,omitempty"`
	BaseOID      string    `json:"baseOid,omitempty"`
	BaseMode     string    `json:"baseMode,omitempty"`
	TargetOID    string    `json:"targetOid,omitempty"`
	TargetSHA256 string    `json:"targetSha256,omitempty"`
	TargetMode   string    `json:"targetMode,omitempty"`
}

// StagingEntry summarizes one changed, untracked, or conflicted path in a worktree.
type StagingEntry struct {
	Path               string            `json:"path"`
	OldPath            string            `json:"oldPath,omitempty"`
	Area               StageArea         `json:"area"`
	Status             string            `json:"status"` // "M","A","D","R","C","T","U","?"
	HunkStageAvailable bool              `json:"hunkStageAvailable"`
	DegradeReason      string            `json:"degradeReason,omitempty"`
	Identity           StageFileIdentity `json:"identity"`
}

// StagingOverview groups worktree changes into staged (HEAD -> index),
// unstaged (index -> worktree, including untracked), and conflicted paths.
type StagingOverview struct {
	Unborn     bool           `json:"unborn"`
	HeadOID    string         `json:"headOid,omitempty"`
	Staged     []StagingEntry `json:"staged"`
	Unstaged   []StagingEntry `json:"unstaged"`
	Conflicted []StagingEntry `json:"conflicted"`
}

// StagingFileDiff returns the content-bound snapshot and stageable hunks for one file.
type StagingFileDiff struct {
	Identity           StageFileIdentity `json:"identity"`
	Conflicted         bool              `json:"conflicted"`
	Binary             bool              `json:"binary"`
	HunkStageAvailable bool              `json:"hunkStageAvailable"`
	DegradeReason      string            `json:"degradeReason,omitempty"`
	Hunks              []StagingHunk     `json:"hunks"`
	baseBytes          []byte
	targetBytes        []byte
	worktreeRaw        []byte
}

// StageRequest requests whole-file (empty HunkIDs) or hunk-subset Stage / Unstage.
type StageRequest struct {
	RepositoryID string            `json:"repositoryId"`
	WorktreeID   string            `json:"worktreeId"`
	Identity     StageFileIdentity `json:"identity"`
	HunkIDs      []string          `json:"hunkIds,omitempty"`
}

func (s *Service) worktreeWriteLock(canonicalPath string) chan struct{} {
	s.writeGatesMu.Lock()
	defer s.writeGatesMu.Unlock()
	if s.writeGates == nil {
		s.writeGates = make(map[string]chan struct{})
	}
	ch, ok := s.writeGates[canonicalPath]
	if !ok {
		ch = make(chan struct{}, 1)
		ch <- struct{}{}
		s.writeGates[canonicalPath] = ch
	}
	return ch
}

func (s *Service) acquireWorktreeWrite(ctx context.Context, canonicalPath string) error {
	ch := s.worktreeWriteLock(canonicalPath)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

func (s *Service) releaseWorktreeWrite(canonicalPath string) {
	ch := s.worktreeWriteLock(canonicalPath)
	ch <- struct{}{}
}

func (s *Service) gitInputOutput(ctx context.Context, cwd string, stdin io.Reader, extra []string, args ...string) ([]byte, string, error) {
	if s.gitPath == "" {
		return nil, "", problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stdout := &boundedOutput{limit: maxWorkspaceGitOutput, cancel: cancel}
	run := runGitProcess(ctx, gitProcess{
		path:        s.gitPath,
		args:        append([]string{"-C", cwd}, args...),
		env:         append(append([]string{}, gitEnv...), extra...),
		stdin:       stdin,
		stdout:      stdout,
		stderrLimit: maxWorkspaceGitError,
	})
	if err := run.err; err != nil {
		if stdout.exceeded || run.stderrExceeded {
			return nil, run.stderr, problemf(ErrTooLarge, "Git 输出超过限制")
		}
		if run.ctxErr != nil {
			return nil, run.stderr, problemf(ErrTimeout, "Git 操作取消或超时: %v", run.ctxErr)
		}
		var launch *exec.Error
		if errors.As(err, &launch) || errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return nil, run.stderr, problemf(ErrUnavailable, "Git 无法执行: %v", err)
		}
		if isGitLockBusy(run.stderr) {
			return nil, run.stderr, problemf(ErrLocked, "仓库正被其他 Git 进程占用，请稍后重试")
		}
		return nil, run.stderr, problemf(ErrInternal, "Git %s 失败: %s (%v)", args[0], strings.TrimSpace(run.stderr), err)
	}
	return stdout.Bytes(), run.stderr, nil
}

// isGitLockBusy detects Git lock file contention on stderr without ever touching lock files.
func isGitLockBusy(stderr string) bool {
	return (strings.Contains(stderr, "Unable to create '") && strings.Contains(stderr, ".lock':")) ||
		strings.Contains(stderr, "Another git process seems to be running")
}

func validateRepoRelPath(rel string) (string, error) {
	if rel == "" || strings.ContainsRune(rel, 0) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "-") || strings.HasPrefix(rel, ":") {
		return "", problemf(ErrInvalid, "文件路径无效")
	}
	cleaned := path.Clean(rel)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != rel {
		return "", problemf(ErrInvalid, "文件路径无效")
	}
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == "" || seg == "." || seg == ".." || seg == ".git" {
			return "", problemf(ErrInvalid, "文件路径无效")
		}
	}
	return cleaned, nil
}

func (s *Service) locateEditableWorktree(ctx context.Context, repositoryID, worktreeID string) (WorktreeLocation, error) {
	if repositoryID == "" || worktreeID == "" {
		return WorktreeLocation{}, problemf(ErrInvalid, "缺少仓库或工作树标识")
	}
	loc, err := s.LocateWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return WorktreeLocation{}, err
	}
	if !loc.Available {
		return WorktreeLocation{}, problemf(ErrReadOnly, "%s", loc.Reason)
	}
	if loc.Kind == "bare" || loc.Path == "" {
		return WorktreeLocation{}, problemf(ErrReadOnly, "bare 仓库没有可操作的工作树索引")
	}
	return loc, nil
}

func (s *Service) resolveCommitReadOnly(ctx context.Context, cwd, rev string) (string, error) {
	if s.gitPath == "" {
		return "", problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	stdout, run := s.runGit(ctx, cwd, []string{"GIT_OPTIONAL_LOCKS=0"}, "rev-parse", "--verify", "-q", "--end-of-options", rev+"^{commit}")
	if run.err != nil {
		return "", nil
	}
	oid := strings.TrimSpace(stdout.String())
	if !isObjectID(oid) {
		return "", nil
	}
	return oid, nil
}

func (s *Service) emptyTreeOID(ctx context.Context, cwd string) (string, error) {
	out, err := ReadGitInput(ctx, cwd, 128, bytes.NewReader(nil), "hash-object", "-t", "tree", "--stdin")
	if err != nil {
		return "", err
	}
	oid := strings.TrimSpace(string(out))
	if !isObjectID(oid) {
		return "", problemf(ErrInternal, "无法计算空树对象 ID")
	}
	return oid, nil
}

// InspectStaging returns all staged, unstaged (including untracked), and conflicted
// entries for the target worktree, allowing partially staged files in both groups.
func (s *Service) InspectStaging(ctx context.Context, repositoryID, worktreeID string) (StagingOverview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return StagingOverview{}, err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return StagingOverview{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)
	return s.inspectStagingLocked(ctx, loc)
}

func (s *Service) inspectStagingLocked(ctx context.Context, loc WorktreeLocation) (StagingOverview, error) {
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	out := StagingOverview{
		Unborn:     headOID == "",
		HeadOID:    headOID,
		Staged:     []StagingEntry{},
		Unstaged:   []StagingEntry{},
		Conflicted: []StagingEntry{},
	}
	data, err := ReadGit(ctx, loc.Path, maxWorkspaceGitOutput, "status", "--porcelain=v2", "-z", "--untracked-files=normal")
	if err != nil {
		return StagingOverview{}, err
	}
	if len(data) == 0 {
		return out, nil
	}
	records := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	for i := 0; i < len(records); i++ {
		rec := string(records[i])
		if len(rec) < 3 {
			continue
		}
		switch rec[0] {
		case 'u':
			fields := strings.SplitN(rec, " ", 11)
			if len(fields) < 11 {
				continue
			}
			relPath := fields[10]
			out.Conflicted = append(out.Conflicted, StagingEntry{
				Path:               relPath,
				Area:               StageAreaConflicted,
				Status:             "U",
				HunkStageAvailable: false,
				DegradeReason:      "文件存在未解决的合并冲突，请先处理冲突",
				Identity:           StageFileIdentity{Area: StageAreaConflicted, Path: relPath},
			})
		case '1', '2':
			limit := 9
			if rec[0] == '2' {
				limit = 10
			}
			fields := strings.SplitN(rec, " ", limit)
			if len(fields) < limit {
				continue
			}
			xy := fields[1]
			relPath := fields[limit-1]
			oldPath := ""
			if rec[0] == '2' && i+1 < len(records) {
				i++
				oldPath = string(records[i])
			}
			if len(xy) == 2 && xy[0] != '.' {
				diff, dErr := s.inspectFileLocked(ctx, loc, headOID, StageAreaStaged, relPath, oldPath, string(xy[0]))
				if dErr == nil {
					out.Staged = append(out.Staged, StagingEntry{
						Path:               relPath,
						OldPath:            oldPath,
						Area:               StageAreaStaged,
						Status:             string(xy[0]),
						HunkStageAvailable: diff.HunkStageAvailable,
						DegradeReason:      diff.DegradeReason,
						Identity:           diff.Identity,
					})
				}
			}
			if len(xy) == 2 && xy[1] != '.' {
				diff, dErr := s.inspectFileLocked(ctx, loc, headOID, StageAreaUnstaged, relPath, "", string(xy[1]))
				if dErr == nil {
					out.Unstaged = append(out.Unstaged, StagingEntry{
						Path:               relPath,
						Area:               StageAreaUnstaged,
						Status:             string(xy[1]),
						HunkStageAvailable: diff.HunkStageAvailable,
						DegradeReason:      diff.DegradeReason,
						Identity:           diff.Identity,
					})
				}
			}
		case '?':
			relPath := rec[2:]
			diff, dErr := s.inspectFileLocked(ctx, loc, headOID, StageAreaUnstaged, relPath, "", "?")
			if dErr == nil {
				out.Unstaged = append(out.Unstaged, StagingEntry{
					Path:               relPath,
					Area:               StageAreaUnstaged,
					Status:             "?",
					HunkStageAvailable: diff.HunkStageAvailable,
					DegradeReason:      diff.DegradeReason,
					Identity:           diff.Identity,
				})
			}
		}
	}
	return out, nil
}
