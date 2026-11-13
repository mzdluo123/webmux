package workspace

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
)

const unstashRefPrefix = "refs/webmux/unstash/"

// ListStashes lists all stash entries in refs/stash reflog for a registered repository.
func (s *Service) ListStashes(ctx context.Context, repositoryID string) ([]StashEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(repositoryID)
	if err != nil {
		return nil, err
	}
	if err := acquire(ctx, r); err != nil {
		return nil, err
	}
	defer release(r)
	if err := s.verifyIdentity(ctx, r); err != nil {
		return nil, err
	}
	return s.listStashesLocked(ctx, r.entry.AnchorPath)
}

func (s *Service) listStashesLocked(ctx context.Context, anchor string) ([]StashEntry, error) {
	topOID, err := s.resolveCommitReadOnly(ctx, anchor, "refs/stash")
	if err != nil || topOID == "" {
		return []StashEntry{}, nil
	}
	data, err := ReadGit(ctx, anchor, maxHistoryOutput,
		"log", "-g", "--no-show-signature",
		"--format=%H%x00%gd%x00%P%x00%gs%x00%cI%x00",
		"refs/stash", "--",
	)
	if err != nil {
		return nil, err
	}
	entries := make([]StashEntry, 0)
	if len(data) == 0 {
		return entries, nil
	}
	fields := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	for i := 0; i+4 < len(fields); i += 5 {
		oid := strings.TrimSpace(string(fields[i]))
		stashRef := strings.TrimSpace(string(fields[i+1]))
		rawParents := strings.TrimSpace(string(fields[i+2]))
		summary := strings.TrimSpace(string(fields[i+3]))
		createdAt := strings.TrimSpace(string(fields[i+4]))
		if !isObjectID(oid) {
			continue
		}
		idx, ok := parseStashIndex(stashRef)
		if !ok {
			idx = len(entries)
			stashRef = fmt.Sprintf("stash@{%d}", idx)
		}
		parents := strings.Fields(rawParents)
		if parents == nil {
			parents = []string{}
		}
		var baseOID, indexOID, untrackedOID string
		if len(parents) > 0 {
			baseOID = parents[0]
		}
		if len(parents) > 1 {
			indexOID = parents[1]
		}
		if len(parents) > 2 {
			untrackedOID = parents[2]
		}
		entries = append(entries, StashEntry{
			StashRef:     stashRef,
			Index:        idx,
			OID:          oid,
			ShortOID:     shortOID(oid),
			BaseOID:      baseOID,
			IndexOID:     indexOID,
			UntrackedOID: untrackedOID,
			Parents:      parents,
			HasUntracked: len(parents) >= 3,
			Branch:       parseStashBranch(summary),
			Summary:      summary,
			CreatedAt:    createdAt,
		})
	}
	return entries, nil
}

func parseStashIndex(stashRef string) (int, bool) {
	rest, ok := strings.CutPrefix(strings.TrimPrefix(stashRef, "refs/"), "stash@{")
	if !ok {
		return 0, false
	}
	numStr, ok := strings.CutSuffix(rest, "}")
	if !ok || numStr == "" {
		return 0, false
	}
	idx, err := strconv.Atoi(numStr)
	if err != nil || idx < 0 {
		return 0, false
	}
	return idx, true
}

func parseStashBranch(summary string) string {
	for _, prefix := range []string{"WIP on ", "On "} {
		if rest, ok := strings.CutPrefix(summary, prefix); ok {
			if branch, _, found := strings.Cut(rest, ":"); found {
				branch = strings.TrimSpace(branch)
				if branch != "" && branch != "(no branch)" {
					return branch
				}
			}
		}
	}
	return ""
}

// InspectStash returns full metadata and file list (including 3rd-parent untracked
// entries) for a verified stash commit OID.
func (s *Service) InspectStash(ctx context.Context, repositoryID, stashOID string) (StashDetail, error) {
	if !isObjectID(stashOID) {
		return StashDetail{}, problemf(ErrInvalid, "stashOid 必须是完整的十六进制提交 OID")
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(repositoryID)
	if err != nil {
		return StashDetail{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return StashDetail{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(ctx, r); err != nil {
		return StashDetail{}, err
	}
	return s.inspectStashLocked(ctx, r.entry.AnchorPath, stashOID)
}

func (s *Service) inspectStashLocked(ctx context.Context, anchor, stashOID string) (StashDetail, error) {
	stashes, err := s.listStashesLocked(ctx, anchor)
	if err != nil {
		return StashDetail{}, err
	}
	var found *StashEntry
	for i := range stashes {
		if stashes[i].OID == stashOID {
			found = &stashes[i]
			break
		}
	}
	if found == nil {
		return StashDetail{}, problemf(ErrNotFound, "指定的贮藏条目不存在或已被移除")
	}

	metaRaw, err := ReadGit(ctx, anchor, 8192, "log", "-1", "--no-show-signature", "--format=%an%x00%ae%x00", stashOID, "--")
	if err != nil {
		return StashDetail{}, err
	}
	metaParts := bytes.Split(bytes.TrimSuffix(metaRaw, []byte{0}), []byte{0})
	var authorName, authorEmail string
	if len(metaParts) >= 2 {
		authorName = strings.TrimSpace(string(metaParts[0]))
		authorEmail = strings.TrimSpace(string(metaParts[1]))
	}

	files := make([]StashFileChange, 0)
	trackedCount := 0
	if found.BaseOID != "" {
		tracked, tErr := readCommitFileChanges(ctx, anchor, found.BaseOID, stashOID)
		if tErr != nil {
			return StashDetail{}, tErr
		}
		for _, f := range tracked {
			files = append(files, StashFileChange{
				OldPath:     f.OldPath,
				NewPath:     f.NewPath,
				Status:      f.Status,
				OldMode:     f.OldMode,
				NewMode:     f.NewMode,
				OldOID:      f.OldOID,
				NewOID:      f.NewOID,
				Binary:      f.Binary,
				ModeChanged: f.ModeChanged,
				Untracked:   false,
			})
			trackedCount++
		}
	}

	untrackedCount := 0
	if found.UntrackedOID != "" {
		untrackedFiles, uErr := s.readUntrackedParentFiles(ctx, anchor, found.UntrackedOID)
		if uErr != nil {
			return StashDetail{}, uErr
		}
		files = append(files, untrackedFiles...)
		untrackedCount = len(untrackedFiles)
	}

	return StashDetail{
		Entry:          *found,
		AuthorName:     authorName,
		AuthorEmail:    authorEmail,
		TrackedCount:   trackedCount,
		UntrackedCount: untrackedCount,
		Files:          files,
	}, nil
}

func (s *Service) readUntrackedParentFiles(ctx context.Context, cwd, untrackedOID string) ([]StashFileChange, error) {
	raw, err := ReadGit(ctx, cwd, maxHistoryOutput, "ls-tree", "-r", "-z", untrackedOID)
	if err != nil {
		return nil, err
	}
	out := make([]StashFileChange, 0)
	if len(raw) == 0 {
		return out, nil
	}
	zeroOID := strings.Repeat("0", len(untrackedOID))
	for _, rec := range bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		header, filePath, ok := strings.Cut(string(rec), "\t")
		if !ok || filePath == "" {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] == "tree" {
			continue
		}
		mode, blobOID := fields[0], fields[2]
		binary := mode == "160000" || s.classifyBlobKind(ctx, cwd, blobOID) == ConflictKindBinary
		out = append(out, StashFileChange{
			OldPath:   filePath,
			NewPath:   filePath,
			Status:    "A",
			OldMode:   "000000",
			NewMode:   mode,
			OldOID:    zeroOID,
			NewOID:    blobOID,
			Binary:    binary,
			Untracked: true,
		})
	}
	return out, nil
}

func (s *Service) resolveStashSlotLocked(ctx context.Context, anchor, stashRef, stashOID string) (StashEntry, error) {
	idx, ok := parseStashIndex(stashRef)
	if !ok {
		return StashEntry{}, problemf(ErrInvalid, "stashRef 格式无效，应为 stash@{n}")
	}
	if !isObjectID(stashOID) {
		return StashEntry{}, problemf(ErrInvalid, "stashOid 必须是完整的十六进制提交 OID")
	}
	stashes, err := s.listStashesLocked(ctx, anchor)
	if err != nil {
		return StashEntry{}, err
	}
	canonicalRef := fmt.Sprintf("stash@{%d}", idx)
	for _, entry := range stashes {
		if entry.StashRef == canonicalRef || entry.Index == idx {
			if entry.OID != stashOID {
				return StashEntry{}, problemf(ErrStale, "贮藏列表已发生变化（%s 当前指向 %s 而非预览时的 %s），请刷新列表后重试", canonicalRef, shortOID(entry.OID), shortOID(stashOID))
			}
			return entry, nil
		}
	}
	for _, entry := range stashes {
		if entry.OID == stashOID {
			return StashEntry{}, problemf(ErrStale, "贮藏条目位置已由 %s 偏移至 %s，请刷新列表后重试", canonicalRef, entry.StashRef)
		}
	}
	return StashEntry{}, problemf(ErrNotFound, "目标贮藏条目 %s 不存在或已被移除", canonicalRef)
}

func (s *Service) writeUnstashRefBestEffort(ctx context.Context, cwd, worktreeID, stashOID string) {
	if worktreeID == "" || !isObjectID(stashOID) || s.gitPath == "" {
		return
	}
	_, _ = s.git(ctx, cwd, "update-ref", unstashRefPrefix+worktreeID, stashOID)
}

func (s *Service) readUnstashRefOID(ctx context.Context, cwd, worktreeID string) string {
	if worktreeID == "" {
		return ""
	}
	oid, err := s.resolveCommitReadOnly(ctx, cwd, unstashRefPrefix+worktreeID)
	if err != nil || !isObjectID(oid) {
		return ""
	}
	return oid
}

func (s *Service) clearUnstashRefBestEffort(ctx context.Context, cwd, worktreeID string) {
	if worktreeID == "" || s.gitPath == "" {
		return
	}
	if s.readUnstashRefOID(ctx, cwd, worktreeID) == "" {
		return
	}
	_, _ = s.git(ctx, cwd, "update-ref", "-d", unstashRefPrefix+worktreeID)
}

func (s *Service) populateUnstashOperationMeta(ctx context.Context, cwd, worktreeID string, out *InProgressOperation) {
	out.TargetLabel = firstNonEmpty(out.CurrentBranch, "HEAD")
	out.TargetOID = out.HeadOID
	out.TargetSubject = out.HeadSubject
	if out.OrigHeadOID == "" && out.HeadOID != "" {
		out.OrigHeadOID = out.HeadOID
		out.OrigHeadSubject = out.HeadSubject
	}
	stashOID := s.readUnstashRefOID(ctx, cwd, worktreeID)
	if stashOID == "" {
		stashOID, _ = s.resolveCommitReadOnly(ctx, cwd, "refs/stash")
	}
	if isObjectID(stashOID) {
		out.SourceOID = stashOID
		out.SourceSubject = s.readCommitSubjectBestEffort(ctx, cwd, stashOID)
		out.SourceLabel = "贮藏变更 " + shortOID(stashOID)
	} else {
		out.SourceLabel = "贮藏变更"
	}
}
