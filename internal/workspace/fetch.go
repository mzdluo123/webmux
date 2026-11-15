package workspace

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// fetchRemoteTimeout bounds the fetch of one remote by default.
const fetchRemoteTimeout = 60 * time.Second

// fetchRemotesTimeout bounds a whole FetchRemotes run; remotes it leaves
// unfetched are reported as timed out.
const fetchRemotesTimeout = 3 * time.Minute

// FetchResult is the outcome of fetching one remote; Message is empty when
// Status is "ok".
type FetchResult struct {
	Remote  string `json:"remote"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// FetchRequest carries optional client idempotency and worktree context.
type FetchRequest struct {
	ClientRequestID string `json:"clientRequestId,omitempty"`
	WorktreeID      string `json:"worktreeId,omitempty"`
}

// FetchRemotesResult lists each remote's outcome and the repository as it
// is after the fetch.
type FetchRemotesResult struct {
	OperationID string        `json:"operationId,omitempty"`
	Results     []FetchResult `json:"results"`
	Repository  Repository    `json:"repository"`
}

// fetchFlight is a running FetchRemotes shared by every caller that asks
// meanwhile. Unlike a snapshot it is not cancelled when its callers leave:
// an interrupted fetch only makes the user retry.
type fetchFlight struct {
	done   chan struct{}
	result FetchRemotesResult
	err    error
}

// FetchRemotes fetches every remote of the repository in name order with
// pruning, then rereads the repository. One remote's failure does not stop
// the others. It holds no repository gate, so worktree operations and
// snapshots go on meanwhile; a caller that leaves does not stop it.
func (s *Service) FetchRemotes(ctx context.Context, repositoryID string) (FetchRemotesResult, error) {
	return s.FetchRemotesWithOptions(ctx, repositoryID, FetchRequest{})
}

// FetchRemotesWithOptions executes FetchRemotes with optional clientRequestId
// idempotency and ledger recording.
func (s *Service) FetchRemotesWithOptions(ctx context.Context, repositoryID string, req FetchRequest) (FetchRemotesResult, error) {
	reqHash := hashOperationRequest(GitOpFetch, repositoryID, req.WorktreeID)
	return runIdempotentOperation(
		ctx, s, repositoryID, req.ClientRequestID, GitOpFetch, reqHash,
		func(op GitOperation) (FetchRemotesResult, error) {
			return s.replayFetchOperation(repositoryID, op)
		},
		func(opID string) (FetchRemotesResult, error) {
			return s.fetchRemotesOnce(ctx, repositoryID, req, opID, reqHash)
		},
	)
}

func (s *Service) fetchRemotesOnce(ctx context.Context, repositoryID string, req FetchRequest, opID, reqHash string) (FetchRemotesResult, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return FetchRemotesResult{}, err
	}
	r.fetchMu.Lock()
	flight := r.fetching
	startedFlight := false
	if flight == nil {
		flight = &fetchFlight{done: make(chan struct{})}
		r.fetching = flight
		startedFlight = true
		runCtx := context.WithoutCancel(ctx)
		go func() {
			flight.result, flight.err = s.fetchRemotes(runCtx, r, opID, req, reqHash)
			r.fetchMu.Lock()
			r.fetching = nil
			r.fetchMu.Unlock()
			close(flight.done)
		}()
	}
	r.fetchMu.Unlock()
	select {
	case <-ctx.Done():
		return FetchRemotesResult{}, ctx.Err()
	case <-flight.done:
		if flight.err != nil {
			return FetchRemotesResult{}, flight.err
		}
		if !startedFlight && req.ClientRequestID != "" {
			now := time.Now().UTC()
			status, summary := summarizeFetchResults(flight.result.Results)
			_ = s.recordOperationStart(GitOperation{
				ID:              opID,
				RepositoryID:    repositoryID,
				WorktreeID:      req.WorktreeID,
				ClientRequestID: req.ClientRequestID,
				RequestHash:     reqHash,
				Kind:            GitOpFetch,
				Status:          status,
				StartedAt:       now,
				FinishedAt:      &now,
				Summary:         summary,
				FetchResults:    append([]FetchResult(nil), flight.result.Results...),
			})
			res := flight.result
			res.OperationID = opID
			return res, nil
		}
		return flight.result, nil
	}
}

func (s *Service) fetchRemotes(ctx context.Context, r *repositoryState, opID string, req FetchRequest, reqHash string) (FetchRemotesResult, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchRemotesTimeout)
	defer cancel()
	anchor := r.entry.AnchorPath
	remotes, err := s.remoteNames(ctx, r)
	if err != nil {
		return FetchRemotesResult{}, err
	}
	if len(remotes) == 0 {
		return FetchRemotesResult{}, problemf(ErrConflict, "仓库没有配置远程，无法获取")
	}
	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    r.entry.ID,
		WorktreeID:      req.WorktreeID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpFetch,
		Status:          GitOpRunning,
		StartedAt:       time.Now().UTC(),
		Summary:         "正在获取远程更新",
	}); err != nil {
		return FetchRemotesResult{}, err
	}
	if shouldAbortBeforeGit(ctx, opID) {
		return FetchRemotesResult{}, errSimulatedCrash
	}
	timeout := s.fetchRemoteTimeout
	if timeout <= 0 {
		timeout = fetchRemoteTimeout
	}
	results := make([]FetchResult, 0, len(remotes))
	outLines := make([]string, 0, len(remotes))
	for _, remote := range remotes {
		result := FetchResult{Remote: remote, Status: remoteOK}
		var failure *RemoteFailure
		if ctx.Err() != nil {
			failure = &RemoteFailure{Status: remoteTimeout, Message: remoteMessage(remoteTimeoutText, "")}
		} else {
			remoteCtx, remoteCancel := context.WithTimeout(ctx, timeout)
			_, failure = s.remoteGit(remoteCtx, anchor, "fetch", "--prune", "--recurse-submodules=no", "--end-of-options", remote)
			remoteCancel()
		}
		if failure != nil {
			result.Status, result.Message = failure.Status, failure.Message
			outLines = append(outLines, remote+": "+failure.Message)
			// URLs and Git output stay out of the log: they can hold credentials.
			slog.Warn("remote fetch failed", "repository", r.entry.ID, "remote", remote, "status", failure.Status)
		}
		results = append(results, result)
	}
	if shouldAbortAfterGit(ctx, opID) {
		return FetchRemotesResult{}, errSimulatedCrash
	}
	status, summary := summarizeFetchResults(results)
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = status
		op.Summary = summary
		op.FetchResults = append([]FetchResult(nil), results...)
		op.SanitizedOutput = strings.Join(outLines, "\n")
	})
	return FetchRemotesResult{OperationID: opID, Results: results, Repository: s.fetchedRepository(r)}, nil
}

func summarizeFetchResults(results []FetchResult) (string, string) {
	okCount := 0
	var firstFailure string
	for _, item := range results {
		if item.Status == remoteOK {
			okCount++
		} else if firstFailure == "" {
			firstFailure = item.Message
		}
	}
	switch {
	case okCount == len(results):
		return GitOpSucceeded, "已完成远程获取"
	case okCount > 0:
		return GitOpNeedsReview, "部分远程获取失败：" + firstFailure
	default:
		if firstFailure != "" {
			return GitOpFailed, firstFailure
		}
		return GitOpFailed, "远程获取失败"
	}
}

// remoteNames lists the configured remotes, sorted by name.
func (s *Service) remoteNames(ctx context.Context, r *repositoryState) ([]string, error) {
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if err := s.verifyIdentity(readCtx, r); err != nil {
		return nil, err
	}
	data, err := s.git(readCtx, r.entry.AnchorPath, "remote", "-v")
	if err != nil {
		return nil, err
	}
	remotes := parseRemotes(string(data))
	names := make([]string, len(remotes))
	for i, remote := range remotes {
		names[i] = remote.Name
	}
	return names, nil
}

// fetchedRepository rereads the repository after a fetch. It does not join
// a shared snapshot, which may have started before the fetch ended.
func (s *Service) fetchedRepository(r *repositoryState) Repository {
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	if err := acquire(ctx, r); err != nil {
		return s.unavailableRepository(r, err)
	}
	defer release(r)
	return s.readRepository(ctx, r)
}
