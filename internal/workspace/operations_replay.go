package workspace

import "context"

func replayCommitOperation(op GitOperation) (CommitResult, error) {
	switch op.Status {
	case GitOpSucceeded:
		return CommitResult{
			OperationID:     op.ID,
			Outcome:         CommitOutcomeMatched,
			CommitOID:       op.ActualOID,
			Parents:         op.Parents,
			TreeOID:         op.ActualTreeOID,
			ExpectedTreeOID: op.Preview.ExpectedTreeOID,
			Subject:         op.Subject,
			ReviewReasons:   op.ReviewReasons,
		}, nil
	case GitOpNeedsReview:
		return CommitResult{
			OperationID:     op.ID,
			Outcome:         CommitOutcomeNeedsReview,
			CommitOID:       op.ActualOID,
			Parents:         op.Parents,
			TreeOID:         op.ActualTreeOID,
			ExpectedTreeOID: op.Preview.ExpectedTreeOID,
			Subject:         op.Subject,
			ReviewReasons:   op.ReviewReasons,
		}, nil
	case GitOpUncertain:
		reasons := op.ReviewReasons
		if len(reasons) == 0 && op.Summary != "" {
			reasons = []string{op.Summary}
		}
		return CommitResult{
			OperationID:     op.ID,
			Outcome:         CommitOutcomeUncertain,
			CommitOID:       op.ActualOID,
			Parents:         op.Parents,
			TreeOID:         op.ActualTreeOID,
			ExpectedTreeOID: op.Preview.ExpectedTreeOID,
			Subject:         op.Subject,
			ReviewReasons:   reasons,
		}, nil
	default:
		return CommitResult{}, replayFailureError(op)
	}
}

func (s *Service) replayPushOperation(repositoryID string, op GitOperation) (PushResult, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return PushResult{}, err
	}
	if op.RemoteStatus != "" {
		msg := ""
		if op.RemoteStatus != remoteOK || op.Status == GitOpNeedsReview {
			msg = op.Summary
		}
		return PushResult{
			OperationID: op.ID,
			Status:      op.RemoteStatus,
			Message:     msg,
			Repository:  s.fetchedRepository(r),
		}, nil
	}
	if op.Status == GitOpUncertain {
		return PushResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return PushResult{}, replayFailureError(op)
}

func (s *Service) replayFetchOperation(repositoryID string, op GitOperation) (FetchRemotesResult, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return FetchRemotesResult{}, err
	}
	if len(op.FetchResults) > 0 {
		return FetchRemotesResult{
			OperationID: op.ID,
			Results:     append([]FetchResult(nil), op.FetchResults...),
			Repository:  s.fetchedRepository(r),
		}, nil
	}
	if op.Status == GitOpUncertain {
		return FetchRemotesResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return FetchRemotesResult{}, replayFailureError(op)
}

func (s *Service) replayFastForwardOperation(repositoryID string, op GitOperation) (FastForwardResult, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return FastForwardResult{}, err
	}
	if op.Status == GitOpSucceeded {
		return FastForwardResult{
			OperationID: op.ID,
			Repository:  s.fetchedRepository(r),
		}, nil
	}
	if op.Status == GitOpUncertain {
		return FastForwardResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return FastForwardResult{}, replayFailureError(op)
}

func (s *Service) replaySwitchOperation(ctx context.Context, repositoryID, worktreeID string, op GitOperation) (SwitchResult, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return SwitchResult{}, err
	}
	if op.Status == GitOpSucceeded {
		readCtx, cancel := context.WithTimeout(ctx, readTimeout)
		defer cancel()
		if err := acquire(readCtx, r); err != nil {
			return SwitchResult{}, err
		}
		defer release(r)
		tree, err := s.currentTree(readCtx, r, worktreeID)
		if err != nil {
			return SwitchResult{}, err
		}
		return SwitchResult{
			Worktree:    s.withLastCommit(readCtx, r, tree),
			OperationID: op.ID,
		}, nil
	}
	if op.Status == GitOpUncertain {
		return SwitchResult{}, problemf(ErrUncertain, "%s", op.Summary)
	}
	return SwitchResult{}, replayFailureError(op)
}

func fmtBool(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
