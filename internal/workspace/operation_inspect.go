package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxGitControlFile = 64 << 10

func readTrimmedGitFile(gitDir string, rel ...string) string {
	full := filepath.Join(append([]string{gitDir}, rel...)...)
	f, err := os.Open(full)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxGitControlFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func checkWorktreeIndexLock(gitDir string) error {
	if _, err := os.Lstat(filepath.Join(gitDir, "index."+"lock")); err == nil {
		return problemf(ErrLocked, "工作树索引锁文件正被其他 Git 进程占用，未删除任何锁文件，请稍后重试")
	}
	return nil
}

// InspectInProgressOperation inspects the live Git sequencer / merge markers and
// unmerged index entries of a worktree on disk (including CLI-started conflicts).
func (s *Service) InspectInProgressOperation(ctx context.Context, repositoryID, worktreeID string) (InProgressOperation, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return InProgressOperation{}, err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return InProgressOperation{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	return s.inspectInProgressLocked(ctx, loc)
}

func (s *Service) inspectInProgressLocked(ctx context.Context, loc WorktreeLocation) (InProgressOperation, error) {
	gitDir, err := worktreeGitDir(loc.Path)
	if err != nil {
		return InProgressOperation{}, problemf(ErrStale, "无法定位工作树 Git 目录: %v", err)
	}
	branch, detached, _ := s.symbolicRefHead(ctx, loc.Path)
	headOID, _ := s.resolveCommitReadOnly(ctx, loc.Path, "HEAD")
	headSubject := s.readCommitSubjectBestEffort(ctx, loc.Path, headOID)

	out := InProgressOperation{
		WorktreeID:       loc.WorktreeID,
		WorktreePath:     loc.Path,
		CurrentBranch:    branch,
		Detached:         detached,
		HeadOID:          headOID,
		HeadSubject:      headSubject,
		ConflictedFiles:  []ConflictFileSummary{},
		ContinueBlockers: []Issue{},
	}

	conflicted, err := s.listConflictedFilesLocked(ctx, loc.Path)
	if err != nil {
		return InProgressOperation{}, err
	}
	out.ConflictedFiles = conflicted
	out.ConflictedCount = len(conflicted)

	if op := detectOperation(loc.Path); op != nil {
		out.Active = true
		out.Kind = *op
	} else if len(conflicted) > 0 {
		out.Active = true
		out.Kind = "merge"
	}

	if !out.Active {
		return out, nil
	}

	origHead := readTrimmedGitFile(gitDir, "ORIG_HEAD")
	if isObjectID(origHead) {
		out.OrigHeadOID = origHead
		out.OrigHeadSubject = s.readCommitSubjectBestEffort(ctx, loc.Path, origHead)
	}

	switch out.Kind {
	case "merge":
		s.populateMergeOperationMeta(ctx, loc.Path, gitDir, &out)
	case "rebase":
		s.populateRebaseOperationMeta(ctx, loc.Path, gitDir, &out)
	case "cherry-pick":
		s.populateSequencerOperationMeta(ctx, loc.Path, gitDir, "CHERRY_PICK_HEAD", "遴选提交", &out)
	case "revert":
		s.populateSequencerOperationMeta(ctx, loc.Path, gitDir, "REVERT_HEAD", "还原提交", &out)
	}

	out.CanAbort = out.Kind == "merge" || out.Kind == "rebase" || out.Kind == "cherry-pick" || out.Kind == "revert"
	if out.Kind == "bisect" {
		out.ContinueBlockers = append(out.ContinueBlockers, Issue{
			Code:    "unsupported_operation",
			Message: "bisect 仅支持在终端处理",
		})
		return out, nil
	}
	if _, lErr := os.Lstat(filepath.Join(gitDir, "index."+"lock")); lErr == nil {
		out.ContinueBlockers = append(out.ContinueBlockers, Issue{
			Code:    "locked",
			Message: "工作树索引锁文件正被其他进程占用",
		})
	}
	if out.ConflictedCount > 0 {
		out.ContinueBlockers = append(out.ContinueBlockers, Issue{
			Code:    "conflicted",
			Message: fmt.Sprintf("仍有 %d 个冲突文件尚未标记为已解决", out.ConflictedCount),
		})
	}
	out.CanContinue = len(out.ContinueBlockers) == 0 && out.CanAbort
	return out, nil
}

func (s *Service) populateMergeOperationMeta(ctx context.Context, cwd, gitDir string, out *InProgressOperation) {
	if out.OrigHeadOID == "" && out.HeadOID != "" {
		out.OrigHeadOID = out.HeadOID
		out.OrigHeadSubject = out.HeadSubject
	}
	out.TargetLabel = out.CurrentBranch
	if out.TargetLabel == "" {
		out.TargetLabel = "HEAD"
	}
	out.TargetOID = out.HeadOID
	out.TargetSubject = out.HeadSubject

	mergeHead := firstTokenLine(readTrimmedGitFile(gitDir, "MERGE_HEAD"))
	if isObjectID(mergeHead) {
		out.SourceOID = mergeHead
		out.SourceSubject = s.readCommitSubjectBestEffort(ctx, cwd, mergeHead)
	}
	mergeMsg := readTrimmedGitFile(gitDir, "MERGE_MSG")
	out.SourceLabel = parseMergeSourceFromMsg(mergeMsg, out.SourceOID)
}

func parseMergeSourceFromMsg(msg, fallbackOID string) string {
	firstLine, _, _ := strings.Cut(msg, "\n")
	firstLine = strings.TrimSpace(firstLine)
	for _, prefix := range []string{"Merge remote-tracking branch '", "Merge branch '", "Merge tag '", "Merge commit '"} {
		if rest, ok := strings.CutPrefix(firstLine, prefix); ok {
			if name, _, found := strings.Cut(rest, "'"); found && name != "" {
				return name
			}
		}
	}
	if fallbackOID != "" {
		return shortOID(fallbackOID)
	}
	return "合入来源"
}

func (s *Service) populateRebaseOperationMeta(ctx context.Context, cwd, gitDir string, out *InProgressOperation) {
	dirName := "rebase-merge"
	if _, err := os.Stat(filepath.Join(gitDir, dirName)); err != nil {
		dirName = "rebase-apply"
	}
	headName := strings.TrimPrefix(readTrimmedGitFile(gitDir, dirName, "head-name"), "refs/heads/")
	onto := firstTokenLine(readTrimmedGitFile(gitDir, dirName, "onto"))
	origHead := firstTokenLine(readTrimmedGitFile(gitDir, dirName, "orig-head"))
	if origHead == "" {
		origHead = firstTokenLine(readTrimmedGitFile(gitDir, dirName, "AbortSafety"))
	}
	if isObjectID(origHead) {
		out.OrigHeadOID = origHead
		out.OrigHeadSubject = s.readCommitSubjectBestEffort(ctx, cwd, origHead)
	}
	if headName != "" {
		out.CurrentBranch = headName
		out.SourceLabel = headName
	} else {
		out.SourceLabel = "当前分支"
	}
	out.SourceOID = out.OrigHeadOID
	out.SourceSubject = out.OrigHeadSubject

	if isObjectID(onto) {
		out.TargetOID = onto
		out.TargetSubject = s.readCommitSubjectBestEffort(ctx, cwd, onto)
		out.TargetLabel = s.describeRefForOID(ctx, cwd, onto)
	}

	stepCurStr := readTrimmedGitFile(gitDir, dirName, "msgnum")
	stepEndStr := readTrimmedGitFile(gitDir, dirName, "end")
	if stepCurStr == "" {
		stepCurStr = readTrimmedGitFile(gitDir, dirName, "next")
		stepEndStr = readTrimmedGitFile(gitDir, dirName, "last")
	}
	if n, err := strconv.Atoi(stepCurStr); err == nil && n > 0 {
		out.StepCurrent = n
	}
	if n, err := strconv.Atoi(stepEndStr); err == nil && n > 0 {
		out.StepTotal = n
	}

	stopped := firstTokenLine(readTrimmedGitFile(gitDir, dirName, "stopped-sha"))
	if stopped == "" {
		stopped = firstTokenLine(readTrimmedGitFile(gitDir, "REBASE_HEAD"))
	}
	if stopped == "" {
		stopped = firstTokenLine(readTrimmedGitFile(gitDir, dirName, "original-commit"))
	}
	if stopped != "" {
		if resolved, _ := s.resolveCommitReadOnly(ctx, cwd, stopped); resolved != "" {
			stopped = resolved
		}
		out.StoppedCommitOID = stopped
		out.StoppedSubject = s.readCommitSubjectBestEffort(ctx, cwd, stopped)
	}
}

func (s *Service) populateSequencerOperationMeta(ctx context.Context, cwd, gitDir, marker, label string, out *InProgressOperation) {
	if out.OrigHeadOID == "" && out.HeadOID != "" {
		out.OrigHeadOID = out.HeadOID
		out.OrigHeadSubject = out.HeadSubject
	}
	out.TargetLabel = out.CurrentBranch
	if out.TargetLabel == "" {
		out.TargetLabel = "HEAD"
	}
	out.TargetOID = out.HeadOID
	out.TargetSubject = out.HeadSubject

	seqOID := firstTokenLine(readTrimmedGitFile(gitDir, marker))
	if isObjectID(seqOID) {
		out.SourceOID = seqOID
		out.SourceSubject = s.readCommitSubjectBestEffort(ctx, cwd, seqOID)
		out.SourceLabel = label + " " + shortOID(seqOID)
		out.StoppedCommitOID = seqOID
		out.StoppedSubject = out.SourceSubject
	}
}

func firstTokenLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (s *Service) readCommitSubjectBestEffort(ctx context.Context, cwd, oid string) string {
	if !isObjectID(oid) {
		return ""
	}
	data, err := ReadGit(ctx, cwd, 4096, "log", "-1", "--no-show-signature", "--format=%s", oid, "--")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (s *Service) describeRefForOID(ctx context.Context, cwd, oid string) string {
	if !isObjectID(oid) {
		return ""
	}
	data, err := ReadGit(ctx, cwd, 16384, "for-each-ref", "--points-at="+oid, "--format=%(refname:short)", "refs/remotes/", "refs/heads/")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasSuffix(line, "/HEAD") {
				return line
			}
		}
	}
	return shortOID(oid)
}

func (s *Service) listConflictedFilesLocked(ctx context.Context, cwd string) ([]ConflictFileSummary, error) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "status", "--porcelain=v2", "-z", "--untracked-files=no")
	if err != nil {
		return nil, err
	}
	out := make([]ConflictFileSummary, 0)
	if len(data) == 0 {
		return out, nil
	}
	for _, rec := range bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0}) {
		line := string(rec)
		if !strings.HasPrefix(line, "u ") {
			continue
		}
		// Format: u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
		fields := strings.SplitN(line, " ", 11)
		if len(fields) < 11 {
			continue
		}
		item := ConflictFileSummary{
			Path:       fields[10],
			XY:         fields[1],
			Stage1Mode: normalizeZeroMode(fields[3]),
			Stage2Mode: normalizeZeroMode(fields[4]),
			Stage3Mode: normalizeZeroMode(fields[5]),
			Stage1OID:  normalizeZeroOID(fields[7]),
			Stage2OID:  normalizeZeroOID(fields[8]),
			Stage3OID:  normalizeZeroOID(fields[9]),
		}
		item.Kind = s.classifyConflictSummaryKind(ctx, cwd, item)
		out = append(out, item)
	}
	return out, nil
}

func normalizeZeroMode(m string) string {
	if m == "" || strings.Trim(m, "0") == "" {
		return ""
	}
	return m
}

func normalizeZeroOID(oid string) string {
	if oid == "" || strings.Trim(oid, "0") == "" {
		return ""
	}
	return oid
}

func (s *Service) classifyConflictSummaryKind(ctx context.Context, cwd string, item ConflictFileSummary) string {
	modes := []string{item.Stage1Mode, item.Stage2Mode, item.Stage3Mode}
	for _, m := range modes {
		if m == "160000" {
			return ConflictKindSubmodule
		}
		if m == "120000" {
			return ConflictKindSymlink
		}
	}
	if item.Stage2Mode == "" || item.Stage3Mode == "" || strings.Contains(item.XY, "D") {
		return ConflictKindDeleteModify
	}
	if s.hasTransformAttributes(ctx, cwd, item.Path) {
		return ConflictKindBinary
	}
	for _, oid := range []string{item.Stage1OID, item.Stage2OID, item.Stage3OID} {
		if oid == "" {
			continue
		}
		kind := s.classifyBlobKind(ctx, cwd, oid)
		if kind != ConflictKindText {
			return kind
		}
	}
	return ConflictKindText
}

func (s *Service) classifyBlobKind(ctx context.Context, cwd, oid string) string {
	sizeOut, err := ReadGit(ctx, cwd, 128, "cat-file", "-s", oid)
	if err == nil {
		if sz, pErr := strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64); pErr == nil && sz > maxHunkFileBytes {
			return ConflictKindTooLarge
		}
	}
	raw, err := ReadGit(ctx, cwd, maxHunkFileBytes+1, "cat-file", "blob", oid)
	if err != nil {
		if errors.Is(err, ErrTooLarge) {
			return ConflictKindTooLarge
		}
		return ConflictKindBinary
	}
	if len(raw) > maxHunkFileBytes {
		return ConflictKindTooLarge
	}
	if bytes.IndexByte(raw, 0) >= 0 || !utf8.Valid(raw) {
		return ConflictKindBinary
	}
	if bytes.Count(raw, []byte("\n"))+1 > maxHunkFileLines {
		return ConflictKindTooLarge
	}
	return ConflictKindText
}
