package workspace

import (
	"bytes"
	"context"
	"strings"
)

func readHistoryRefs(ctx context.Context, anchor string, trees []Worktree) (map[string][]HistoryRefBadge, []string, error) {
	data, err := ReadGit(ctx, anchor, maxWorkspaceGitOutput, "for-each-ref",
		"--format=%(refname)%00%(objectname)%00%(*objectname)%00",
		"refs/heads/", "refs/remotes/", "refs/tags/")
	if err != nil {
		return nil, nil, err
	}
	badges := map[string][]HistoryRefBadge{}
	seenTips := map[string]bool{}
	var tips []string
	addTip := func(oid string) {
		if isObjectID(oid) && !seenTips[oid] {
			seenTips[oid] = true
			tips = append(tips, oid)
		}
	}
	fields := bytes.Split(data, []byte{0})
	for i := 0; i+2 < len(fields); i += 3 {
		ref := strings.TrimPrefix(string(fields[i]), "\n")
		obj := string(fields[i+1])
		peeled := string(fields[i+2])
		if ref == "" || strings.HasSuffix(ref, "/HEAD") {
			continue
		}
		commitOID := obj
		if peeled != "" {
			commitOID = peeled
		}
		if !isObjectID(commitOID) {
			continue
		}
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			badges[commitOID] = append(badges[commitOID], HistoryRefBadge{
				Name: strings.TrimPrefix(ref, "refs/heads/"),
				Kind: "local",
			})
			addTip(commitOID)
		case strings.HasPrefix(ref, "refs/remotes/"):
			badges[commitOID] = append(badges[commitOID], HistoryRefBadge{
				Name: strings.TrimPrefix(ref, "refs/remotes/"),
				Kind: "remote",
			})
			addTip(commitOID)
		case strings.HasPrefix(ref, "refs/tags/"):
			badges[commitOID] = append(badges[commitOID], HistoryRefBadge{
				Name: strings.TrimPrefix(ref, "refs/tags/"),
				Kind: "tag",
			})
			addTip(commitOID)
		}
	}
	for _, wt := range trees {
		if wt.Kind == "bare" || wt.Head == nil || !isObjectID(*wt.Head) {
			continue
		}
		label := "HEAD"
		if wt.Branch != nil && *wt.Branch != "" {
			label = "HEAD -> " + *wt.Branch
		}
		badges[*wt.Head] = append([]HistoryRefBadge{{
			Name:       label,
			Kind:       "head",
			WorktreeID: wt.ID,
		}}, badges[*wt.Head]...)
		addTip(*wt.Head)
	}
	return badges, tips, nil
}

func resolveHistoryScope(ctx context.Context, anchor string, q HistoryQuery, trees []Worktree, allTips []string) (historyScopeSpec, error) {
	var selectedTree *Worktree
	if q.WorktreeID != "" {
		for i := range trees {
			if trees[i].ID == q.WorktreeID {
				selectedTree = &trees[i]
				break
			}
		}
		if selectedTree == nil {
			return historyScopeSpec{}, problemf(ErrNotFound, "工作树不存在，请刷新")
		}
	}

	if q.Branch != "" {
		oid, err := resolveNamedBranchTip(ctx, anchor, q.Branch)
		if err != nil {
			return historyScopeSpec{}, err
		}
		if q.Scope == HistoryScopeAll || q.Scope == HistoryScopeBranch {
			return historyScopeSpec{status: HistoryStatusReady, include: []string{oid}}, nil
		}
		return resolveUpstreamDeltaScope(ctx, anchor, q.Scope, q.Branch, oid)
	}

	switch q.Scope {
	case HistoryScopeAll:
		if len(allTips) == 0 {
			return historyScopeSpec{
				status: HistoryStatusEmpty,
				reason: "当前仓库尚无任何提交（空仓库）",
			}, nil
		}
		return historyScopeSpec{status: HistoryStatusReady, include: allTips}, nil

	case HistoryScopeBranch:
		if selectedTree == nil || selectedTree.Kind == "bare" {
			return historyScopeSpec{
				status: HistoryStatusNoWorktree,
				reason: "未选定可用工作树，无法解析“当前分支”范围；请选择具体分支或切换到“所有历史”",
			}, nil
		}
		if selectedTree.Head == nil || !isObjectID(*selectedTree.Head) {
			return historyScopeSpec{
				status: HistoryStatusUnborn,
				reason: "当前工作树处于未诞生 HEAD 状态，尚无提交记录",
			}, nil
		}
		return historyScopeSpec{status: HistoryStatusReady, include: []string{*selectedTree.Head}}, nil

	case HistoryScopeToPush, HistoryScopeIncoming:
		if selectedTree == nil || selectedTree.Kind == "bare" {
			return historyScopeSpec{
				status: HistoryStatusNoWorktree,
				reason: "未指定目标分支或可用工作树，无法计算与上游的差异",
			}, nil
		}
		if selectedTree.Head == nil || !isObjectID(*selectedTree.Head) {
			return historyScopeSpec{
				status: HistoryStatusUnborn,
				reason: "当前分支尚无提交（未诞生 HEAD），无法计算与上游的差异",
			}, nil
		}
		if selectedTree.Detached || selectedTree.Branch == nil || *selectedTree.Branch == "" {
			return historyScopeSpec{
				status: HistoryStatusDetachedNoRef,
				reason: "当前工作树处于游离 HEAD 状态，没有关联上游跟踪分支",
			}, nil
		}
		return resolveUpstreamDeltaScope(ctx, anchor, q.Scope, *selectedTree.Branch, *selectedTree.Head)
	}
	return historyScopeSpec{}, problemf(ErrInvalid, "未知历史范围")
}

func resolveNamedBranchTip(ctx context.Context, anchor, branch string) (string, error) {
	if strings.HasPrefix(branch, "-") || strings.Contains(branch, "..") {
		return "", problemf(ErrInvalid, "分支名称无效")
	}
	for _, prefix := range []string{"refs/heads/", "refs/remotes/"} {
		out, err := ReadGit(ctx, anchor, 256, "rev-parse", "--verify", "--end-of-options", prefix+branch+"^{commit}")
		if err == nil {
			oid := strings.TrimSpace(string(out))
			if isObjectID(oid) {
				return oid, nil
			}
		}
	}
	return "", problemf(ErrNotFound, "分支 %s 不存在", branch)
}

func resolveUpstreamDeltaScope(ctx context.Context, anchor, scope, branch, localOID string) (historyScopeSpec, error) {
	localRef := "refs/heads/" + branch
	data, err := ReadGit(ctx, anchor, 1024, "for-each-ref", "--format=%(upstream:short)%00%(upstream)", localRef)
	if err != nil {
		return historyScopeSpec{}, err
	}
	short, full, ok := strings.Cut(strings.TrimSuffix(string(data), "\n"), "\x00")
	if !ok || short == "" || full == "" {
		return historyScopeSpec{
			status: HistoryStatusNoUpstream,
			reason: "当前分支尚未配置上游跟踪分支",
		}, nil
	}
	upOut, err := ReadGit(ctx, anchor, 256, "rev-parse", "--verify", "--end-of-options", full+"^{commit}")
	if err != nil {
		return historyScopeSpec{
			status:   HistoryStatusUpstreamGone,
			reason:   "当前分支配置的上游跟踪引用不存在或已被清理，请先执行获取（Fetch）",
			upstream: short,
		}, nil
	}
	upOID := strings.TrimSpace(string(upOut))
	if !isObjectID(upOID) {
		return historyScopeSpec{
			status:   HistoryStatusUpstreamGone,
			reason:   "无法解析上游跟踪引用提交",
			upstream: short,
		}, nil
	}
	if scope == HistoryScopeToPush {
		return historyScopeSpec{
			status:   HistoryStatusReady,
			upstream: short,
			include:  []string{localOID},
			exclude:  []string{upOID},
		}, nil
	}
	return historyScopeSpec{
		status:   HistoryStatusReady,
		upstream: short,
		include:  []string{upOID},
		exclude:  []string{localOID},
	}, nil
}
