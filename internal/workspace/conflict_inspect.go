package workspace

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// InspectConflictFile inspects a single conflicted path in a worktree, building
// either a deterministic 3-way text merge model or dedicated binary/structural
// conflict metadata, while keeping Left = Current Branch and Right = Target /
// Incoming across both Merge and Rebase (git4idea GitMergeProvider semantics).
func (s *Service) InspectConflictFile(ctx context.Context, repositoryID, worktreeID, relPath string) (ConflictFileDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	cleanPath, err := validateRepoRelPath(relPath)
	if err != nil {
		return ConflictFileDetail{}, err
	}
	loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
	if err != nil {
		return ConflictFileDetail{}, err
	}
	if err := s.acquireWorktreeWrite(ctx, loc.Path); err != nil {
		return ConflictFileDetail{}, err
	}
	defer s.releaseWorktreeWrite(loc.Path)

	if err := verifyWorktreeSameOnDisk(loc); err != nil {
		return ConflictFileDetail{}, err
	}
	return s.inspectConflictFileLocked(ctx, repositoryID, worktreeID, loc, cleanPath)
}

func (s *Service) inspectConflictFileLocked(ctx context.Context, repositoryID, worktreeID string, loc WorktreeLocation, cleanPath string) (ConflictFileDetail, error) {
	inProg, err := s.inspectInProgressLocked(ctx, loc)
	if err != nil {
		return ConflictFileDetail{}, err
	}

	var matched *ConflictFileSummary
	for i := range inProg.ConflictedFiles {
		if inProg.ConflictedFiles[i].Path == cleanPath {
			matched = &inProg.ConflictedFiles[i]
			break
		}
	}
	if matched == nil {
		stages, uErr := s.readUnmergedStagesLocked(ctx, loc.Path, cleanPath)
		if uErr != nil {
			return ConflictFileDetail{}, uErr
		}
		if stages.Stage1Mode == "" && stages.Stage2Mode == "" && stages.Stage3Mode == "" {
			return ConflictFileDetail{}, problemf(ErrNotFound, "文件 %s 当前不存在未解决的合并冲突", cleanPath)
		}
		stages.Kind = s.classifyConflictSummaryKind(ctx, loc.Path, stages)
		matched = &stages
	}

	wtMode, wtSum, wtRaw, wtErr := s.readWorktreeFile(ctx, loc.Path, cleanPath, matched.Stage2Mode)
	if wtErr != nil {
		return ConflictFileDetail{}, wtErr
	}

	leftStage, rightStage := 2, 3
	if inProg.Kind == "rebase" {
		// During rebase, Git checks out `onto` (stage 2) and replays the user's
		// feature branch commit (stage 3). Keep Left = Current Branch (stage 3)
		// and Right = Rebase Target Base (stage 2) so columns never invert.
		leftStage, rightStage = 3, 2
	}

	identity := ConflictSnapshotIdentity{
		Path:           cleanPath,
		OperationKind:  inProg.Kind,
		Stage1Mode:     matched.Stage1Mode,
		Stage1OID:      matched.Stage1OID,
		Stage2Mode:     matched.Stage2Mode,
		Stage2OID:      matched.Stage2OID,
		Stage3Mode:     matched.Stage3Mode,
		Stage3OID:      matched.Stage3OID,
		LeftStage:      leftStage,
		RightStage:     rightStage,
		WorktreeMode:   wtMode,
		WorktreeSHA256: wtSum,
	}

	baseSide, baseBytes, err := s.loadConflictSide(ctx, loc.Path, "base", 1, matched.Stage1Mode, matched.Stage1OID, inProg)
	if err != nil {
		return ConflictFileDetail{}, err
	}
	leftMode, leftOID := matched.Stage2Mode, matched.Stage2OID
	rightMode, rightOID := matched.Stage3Mode, matched.Stage3OID
	if leftStage == 3 {
		leftMode, leftOID = matched.Stage3Mode, matched.Stage3OID
		rightMode, rightOID = matched.Stage2Mode, matched.Stage2OID
	}

	leftSide, leftBytes, err := s.loadConflictSide(ctx, loc.Path, "left", leftStage, leftMode, leftOID, inProg)
	if err != nil {
		return ConflictFileDetail{}, err
	}
	rightSide, rightBytes, err := s.loadConflictSide(ctx, loc.Path, "right", rightStage, rightMode, rightOID, inProg)
	if err != nil {
		return ConflictFileDetail{}, err
	}

	detail := ConflictFileDetail{
		RepositoryID:  repositoryID,
		WorktreeID:    worktreeID,
		WorktreePath:  loc.Path,
		Path:          cleanPath,
		XY:            matched.XY,
		Kind:          matched.Kind,
		OperationKind: inProg.Kind,
		Identity:      identity,
		Base:          baseSide,
		Left:          leftSide,
		Right:         rightSide,
		Segments:      []ConflictSegment{},
	}

	if matched.Kind != ConflictKindText {
		detail.TextMergeable = false
		detail.DegradeReason = conflictDegradeReason(matched.Kind, leftSide, rightSide)
		return detail, nil
	}

	segments, initialResult := buildThreeWaySegments(baseBytes, leftBytes, rightBytes)
	detail.TextMergeable = true
	detail.Segments = segments
	detail.InitialResultText = initialResult

	if len(wtRaw) > 0 && bytes.IndexByte(wtRaw, 0) < 0 {
		detail.DiskHasConflictMarker = hasGitConflictMarkers(wtRaw)
		if !detail.DiskHasConflictMarker && len(wtRaw) <= maxHunkFileBytes {
			diskStr := string(wtRaw)
			detail.DiskText = diskStr
			detail.DiskModifiedFromBase = diskStr != initialResult
		}
	}
	return detail, nil
}

func (s *Service) readUnmergedStagesLocked(ctx context.Context, cwd, cleanPath string) (ConflictFileSummary, error) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "ls-files", "-u", "-z", "--", cleanPath)
	if err != nil {
		return ConflictFileSummary{}, err
	}
	out := ConflictFileSummary{Path: cleanPath, XY: "UU"}
	for _, rec := range bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		meta, p, ok := strings.Cut(string(rec), "\t")
		if !ok || p != cleanPath {
			continue
		}
		parts := strings.Fields(meta)
		if len(parts) != 3 {
			continue
		}
		switch parts[2] {
		case "1":
			out.Stage1Mode, out.Stage1OID = parts[0], parts[1]
		case "2":
			out.Stage2Mode, out.Stage2OID = parts[0], parts[1]
		case "3":
			out.Stage3Mode, out.Stage3OID = parts[0], parts[1]
		}
	}
	return out, nil
}

func (s *Service) loadConflictSide(ctx context.Context, cwd, side string, stage int, mode, oid string, inProg InProgressOperation) (ConflictSide, []byte, error) {
	out := ConflictSide{
		Side:    side,
		Stage:   stage,
		Present: mode != "" && oid != "",
		Mode:    mode,
		BlobOID: oid,
	}
	populateConflictSideMeta(&out, side, stage, inProg)
	if out.CommitOID == "" && side == "base" {
		if mb := s.computeConflictMergeBase(ctx, cwd, inProg); mb != "" {
			out.CommitOID = mb
			out.Subject = s.readCommitSubjectBestEffort(ctx, cwd, mb)
		}
	}
	if !out.Present {
		return out, nil, nil
	}
	if mode == "160000" {
		out.Binary = true
		return out, nil, nil
	}
	kind := s.classifyBlobKind(ctx, cwd, oid)
	if kind == ConflictKindTooLarge {
		sizeOut, _ := ReadGit(ctx, cwd, 128, "cat-file", "-s", oid)
		_, _ = fmt.Sscanf(strings.TrimSpace(string(sizeOut)), "%d", &out.SizeBytes)
		out.Binary = true
		return out, nil, nil
	}
	raw, err := ReadGit(ctx, cwd, maxHunkFileBytes+1, "cat-file", "blob", oid)
	if err != nil {
		return ConflictSide{}, nil, err
	}
	out.SizeBytes = len(raw)
	if kind != ConflictKindText || mode == "120000" {
		out.Binary = kind == ConflictKindBinary
		if mode == "120000" {
			out.Text = string(raw)
		}
		return out, raw, nil
	}
	out.Text = string(raw)
	out.LineEnding = detectDominantLineEnding(raw)
	return out, raw, nil
}

func populateConflictSideMeta(out *ConflictSide, side string, stage int, inProg InProgressOperation) {
	switch side {
	case "base":
		out.Role = "共同祖先 (Base)"
		out.Label = "共同祖先 · stage 1"
	case "left":
		switch inProg.Kind {
		case "rebase":
			ref := firstNonEmpty(inProg.CurrentBranch, inProg.SourceLabel, "当前分支")
			commit := firstNonEmpty(inProg.StoppedCommitOID, inProg.SourceOID)
			out.RefName = ref
			out.CommitOID = commit
			out.Subject = firstNonEmpty(inProg.StoppedSubject, inProg.SourceSubject)
			out.Role = "当前分支重放提交 (stage 3)"
			out.Label = formatSideLabel(ref, commit, "当前分支重放提交")
		default:
			ref := firstNonEmpty(inProg.CurrentBranch, inProg.TargetLabel, "HEAD")
			out.RefName = ref
			out.CommitOID = inProg.HeadOID
			out.Subject = inProg.HeadSubject
			out.Role = fmt.Sprintf("当前工作树分支 (stage %d)", stage)
			out.Label = formatSideLabel(ref, inProg.HeadOID, "当前分支")
		}
	case "right":
		switch inProg.Kind {
		case "rebase":
			ref := firstNonEmpty(inProg.TargetLabel, "目标基底")
			out.RefName = ref
			out.CommitOID = inProg.TargetOID
			out.Subject = inProg.TargetSubject
			out.Role = "变基目标基底 (stage 2)"
			out.Label = formatSideLabel(ref, inProg.TargetOID, "变基目标基底")
		case "cherry-pick":
			ref := firstNonEmpty(inProg.SourceLabel, "遴选提交")
			out.RefName = ref
			out.CommitOID = inProg.SourceOID
			out.Subject = inProg.SourceSubject
			out.Role = "遴选提交 (stage 3)"
			out.Label = formatSideLabel(ref, inProg.SourceOID, "遴选来源")
		case "revert":
			ref := firstNonEmpty(inProg.SourceLabel, "还原提交")
			out.RefName = ref
			out.CommitOID = inProg.SourceOID
			out.Subject = inProg.SourceSubject
			out.Role = "还原变更 (stage 3)"
			out.Label = formatSideLabel(ref, inProg.SourceOID, "还原来源")
		case "unstash":
			ref := firstNonEmpty(inProg.SourceLabel, "贮藏变更")
			out.RefName = ref
			out.CommitOID = inProg.SourceOID
			out.Subject = inProg.SourceSubject
			out.Role = fmt.Sprintf("贮藏变更 (Stash · stage %d)", stage)
			out.Label = formatSideLabel(ref, inProg.SourceOID, "贮藏来源")
		default:
			ref := firstNonEmpty(inProg.SourceLabel, "合入来源")
			out.RefName = ref
			out.CommitOID = inProg.SourceOID
			out.Subject = inProg.SourceSubject
			out.Role = fmt.Sprintf("合入目标分支 (stage %d)", stage)
			out.Label = formatSideLabel(ref, inProg.SourceOID, "合入来源")
		}
	}
}

func formatSideLabel(ref, oid, role string) string {
	if ref != "" && oid != "" && !strings.Contains(ref, shortOID(oid)) {
		return fmt.Sprintf("%s · %s（%s）", ref, shortOID(oid), role)
	}
	if ref != "" {
		return fmt.Sprintf("%s（%s）", ref, role)
	}
	if oid != "" {
		return fmt.Sprintf("%s（%s）", shortOID(oid), role)
	}
	return role
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *Service) computeConflictMergeBase(ctx context.Context, cwd string, inProg InProgressOperation) string {
	a, b := inProg.HeadOID, inProg.SourceOID
	if inProg.Kind == "rebase" {
		a, b = inProg.TargetOID, firstNonEmpty(inProg.StoppedCommitOID, inProg.SourceOID)
	}
	if !isObjectID(a) || !isObjectID(b) {
		return ""
	}
	out, err := ReadGit(ctx, cwd, 128, "merge-base", a, b)
	if err != nil {
		return ""
	}
	oid := strings.TrimSpace(string(out))
	if !isObjectID(oid) {
		return ""
	}
	return oid
}

func conflictDegradeReason(kind string, left, right ConflictSide) string {
	switch kind {
	case ConflictKindDeleteModify:
		if !left.Present {
			return "左侧已删除此文件，右侧修改了此文件；请选择保留右侧版本或确认删除文件"
		}
		if !right.Present {
			return "右侧已删除此文件，左侧修改了此文件；请选择保留左侧版本或确认删除文件"
		}
		return "文件存在增删结构冲突，请选择保留一侧版本或删除文件"
	case ConflictKindBinary:
		return "二进制或受过滤器转换的文件无法执行文本三方合并，请显式选择保留左侧版本、保留右侧版本或删除文件"
	case ConflictKindSubmodule:
		return "子模块指针冲突无法按文本合并，请选择保留左侧或右侧子模块提交版本"
	case ConflictKindSymlink:
		return "符号链接目标冲突无法按文本合并，请选择保留左侧链接、保留右侧链接或删除"
	case ConflictKindTooLarge:
		return "文件超过文本三方合并大小上限（2 MiB / 20000 行），请选择保留一侧版本或在终端手动处理"
	default:
		return "该冲突不支持文本三方合并，请选择保留一侧版本"
	}
}
