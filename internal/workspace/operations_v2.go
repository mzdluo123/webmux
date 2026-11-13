package workspace

import "context"

func (s *Service) reconcileV2Operation(ctx context.Context, r *repositoryState, op *GitOperation) {
	anchor := r.entry.AnchorPath
	switch op.Kind {
	case GitOpStashDrop:
		if op.Preview.StashOID != "" {
			stashes, err := s.listStashesLocked(ctx, anchor)
			if err == nil {
				stillPresent := false
				for _, entry := range stashes {
					if entry.OID == op.Preview.StashOID {
						stillPresent = true
						break
					}
				}
				if !stillPresent {
					op.Status = GitOpSucceeded
					op.ActualOID = op.Preview.StashOID
					op.Summary = "服务重启对账确认目标贮藏条目已移除"
					return
				}
			}
		}
	case GitOpBranchRename:
		if op.Preview.Branch != "" && op.Preview.Target != "" {
			_, oldErr := s.resolveCommit(ctx, anchor, "refs/heads/"+op.Preview.Branch)
			newTip, newErr := s.resolveCommit(ctx, anchor, "refs/heads/"+op.Preview.Target)
			if oldErr != nil && newErr == nil && (op.Preview.HeadOID == "" || newTip == op.Preview.HeadOID) {
				op.Status = GitOpSucceeded
				op.ActualOID = newTip
				op.Summary = "服务重启对账确认分支已完成重命名"
				return
			}
		}
	case GitOpTagCreate:
		if op.Preview.Target != "" && op.Preview.TargetOID != "" {
			tip, err := s.resolveCommit(ctx, anchor, "refs/tags/"+op.Preview.Target+"^{commit}")
			if err == nil && tip == op.Preview.TargetOID {
				op.Status = GitOpSucceeded
				op.ActualOID = tip
				op.Summary = "服务重启对账确认标签已创建"
				return
			}
		}
	case GitOpTagDelete:
		if op.Preview.Target != "" {
			if _, err := s.resolveCommit(ctx, anchor, "refs/tags/"+op.Preview.Target+"^{commit}"); err != nil {
				op.Status = GitOpSucceeded
				op.ActualOID = op.Preview.TargetOID
				op.Summary = "服务重启对账确认标签已删除"
				return
			}
		}
	}
	op.Status = GitOpUncertain
	op.Summary = "服务重启时操作状态未能唯一确认，请刷新检查最新仓库状态"
}

func (s *Service) replayStashCreateOperation(ctx context.Context, repositoryID, worktreeID string, op GitOperation) (StashCreateResult, error) {
	if op.Status == GitOpSucceeded {
		r, err := s.repository(repositoryID)
		if err != nil {
			return StashCreateResult{}, err
		}
		readCtx, cancel := context.WithTimeout(ctx, readTimeout)
		defer cancel()
		if err := acquire(readCtx, r); err != nil {
			return StashCreateResult{}, err
		}
		defer release(r)
		stashes, err := s.listStashesLocked(readCtx, r.entry.AnchorPath)
		if err != nil {
			return StashCreateResult{}, err
		}
		entry := StashEntry{
			StashRef: "stash@{0}",
			OID:      op.ActualOID,
			ShortOID: shortOID(op.ActualOID),
			Parents:  []string{},
			Summary:  op.Subject,
		}
		for _, item := range stashes {
			if item.OID == op.ActualOID {
				entry = item
				break
			}
		}
		tree, _ := s.currentTree(readCtx, r, worktreeID)
		return StashCreateResult{
			OperationID: op.ID,
			Entry:       entry,
			Worktree:    s.withLastCommit(readCtx, r, tree),
		}, nil
	}
	if op.Status == GitOpUncertain {
		return StashCreateResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return StashCreateResult{}, replayFailureError(op)
}

func (s *Service) replayStashApplyOperation(ctx context.Context, repositoryID, worktreeID string, op GitOperation) (StashApplyResult, error) {
	if op.Status == GitOpSucceeded || op.Status == GitOpNeedsReview {
		loc, err := s.locateEditableWorktree(ctx, repositoryID, worktreeID)
		if err != nil {
			return StashApplyResult{}, err
		}
		r, err := s.repository(repositoryID)
		if err != nil {
			return StashApplyResult{}, err
		}
		readCtx, cancel := context.WithTimeout(ctx, readTimeout)
		defer cancel()
		if err := acquire(readCtx, r); err != nil {
			return StashApplyResult{}, err
		}
		defer release(r)
		inProg, _ := s.inspectInProgressLocked(readCtx, loc)
		tree, _ := s.currentTree(readCtx, r, worktreeID)
		outcome := StashApplyCompleted
		dropped := op.Kind == GitOpStashPop && op.Status == GitOpSucceeded
		if op.Status == GitOpNeedsReview {
			if op.FailureKind == "conflicted" {
				outcome = StashApplyConflicted
			} else {
				outcome = StashApplyNeedsReview
			}
			dropped = false
		}
		return StashApplyResult{
			OperationID:    op.ID,
			Kind:           op.Kind,
			Outcome:        outcome,
			StashRef:       op.Preview.StashRef,
			StashOID:       op.Preview.StashOID,
			StashDropped:   dropped,
			StashPreserved: !dropped,
			Summary:        op.Summary,
			InProgress:     inProg,
			Worktree:       s.withLastCommit(readCtx, r, tree),
		}, nil
	}
	if op.Status == GitOpUncertain {
		return StashApplyResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return StashApplyResult{}, replayFailureError(op)
}

func (s *Service) replayStashDropOperation(ctx context.Context, repositoryID string, op GitOperation) (StashDropResult, error) {
	if op.Status == GitOpSucceeded {
		stashes, err := s.ListStashes(ctx, repositoryID)
		if err != nil {
			return StashDropResult{}, err
		}
		return StashDropResult{
			OperationID:     op.ID,
			StashRef:        op.Preview.StashRef,
			StashOID:        op.Preview.StashOID,
			RecoveryCommand: "git stash apply " + op.Preview.StashOID,
			Stashes:         stashes,
		}, nil
	}
	if op.Status == GitOpUncertain {
		return StashDropResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return StashDropResult{}, replayFailureError(op)
}

func (s *Service) replayBranchMutationOperation(ctx context.Context, repositoryID string, op GitOperation) (BranchMutationResult, error) {
	if op.Status == GitOpSucceeded {
		r, err := s.repository(repositoryID)
		if err != nil {
			return BranchMutationResult{}, err
		}
		branches, err := s.Branches(ctx, repositoryID)
		if err != nil {
			return BranchMutationResult{}, err
		}
		branch := op.Preview.Branch
		upstream := ""
		if op.Kind == GitOpBranchRename {
			branch = op.Preview.Target
		} else {
			upstream = op.Preview.Target
		}
		return BranchMutationResult{
			OperationID: op.ID,
			Branch:      branch,
			Upstream:    upstream,
			Repository:  s.fetchedRepository(r),
			Branches:    branches,
		}, nil
	}
	if op.Status == GitOpUncertain {
		return BranchMutationResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return BranchMutationResult{}, replayFailureError(op)
}

func (s *Service) replayTagCreateOperation(ctx context.Context, repositoryID string, op GitOperation) (CreateTagResult, error) {
	if op.Status == GitOpSucceeded {
		branches, err := s.Branches(ctx, repositoryID)
		if err != nil {
			return CreateTagResult{}, err
		}
		tag := Tag{
			Name: op.Preview.Target,
			Head: op.Preview.TargetOID,
			Kind: "lightweight",
		}
		for _, item := range branches.Tags {
			if item.Name == op.Preview.Target {
				tag = item
				break
			}
		}
		return CreateTagResult{
			OperationID: op.ID,
			Tag:         tag,
			Branches:    branches,
		}, nil
	}
	if op.Status == GitOpUncertain {
		return CreateTagResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return CreateTagResult{}, replayFailureError(op)
}

func (s *Service) replayTagDeleteOperation(ctx context.Context, repositoryID string, op GitOperation) (DeleteTagResult, error) {
	if op.Status == GitOpSucceeded {
		branches, err := s.Branches(ctx, repositoryID)
		if err != nil {
			return DeleteTagResult{}, err
		}
		return DeleteTagResult{
			OperationID:     op.ID,
			Name:            op.Preview.Target,
			TargetOID:       op.Preview.TargetOID,
			RecoveryCommand: "git tag " + op.Preview.Target + " " + op.Preview.TargetOID,
			Branches:        branches,
		}, nil
	}
	if op.Status == GitOpUncertain {
		return DeleteTagResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return DeleteTagResult{}, replayFailureError(op)
}

func (s *Service) replayRemoteMutationOperation(ctx context.Context, repositoryID string, op GitOperation) (RemoteMutationResult, error) {
	if op.Status == GitOpSucceeded {
		r, err := s.repository(repositoryID)
		if err != nil {
			return RemoteMutationResult{}, err
		}
		rem, err := s.Remotes(ctx, repositoryID)
		if err != nil {
			return RemoteMutationResult{}, err
		}
		return RemoteMutationResult{
			OperationID: op.ID,
			Remotes:     rem.Remotes,
			Repository:  s.fetchedRepository(r),
		}, nil
	}
	if op.Status == GitOpUncertain {
		return RemoteMutationResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return RemoteMutationResult{}, replayFailureError(op)
}
