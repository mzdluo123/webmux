package workspace

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// AddRemoteRequest adds a new remote to a registered repository.
type AddRemoteRequest struct {
	Name            string `json:"name"`
	FetchURL        string `json:"fetchUrl"`
	PushURL         string `json:"pushUrl,omitempty"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// UpdateRemoteRequest updates the fetch and/or push URL of an existing remote.
type UpdateRemoteRequest struct {
	FetchURL        *string `json:"fetchUrl,omitempty"`
	PushURL         *string `json:"pushUrl,omitempty"`
	ClearPushURL    bool    `json:"clearPushUrl,omitempty"`
	ClientRequestID string  `json:"clientRequestId,omitempty"`
}

// RemoteMutationResult returns redacted remotes and updated repository metadata.
type RemoteMutationResult struct {
	OperationID string     `json:"operationId,omitempty"`
	Remotes     []Remote   `json:"remotes"`
	Repository  Repository `json:"repository"`
}

func (s *Service) validateRemoteName(ctx context.Context, anchor, name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, " \t\r\n/\x00") {
		return problemf(ErrInvalid, "远程名称无效（不能为空、不能包含空格或斜杠）")
	}
	if _, err := s.git(ctx, anchor, "check-ref-format", "refs/remotes/"+name+"/head"); err != nil {
		return problemf(ErrInvalid, "远程名称不符合 Git 引用格式规则")
	}
	return nil
}

func validateRemoteURL(raw string) error {
	if raw == "" {
		return problemf(ErrInvalid, "远程地址不能为空")
	}
	if strings.HasPrefix(raw, "-") || strings.ContainsAny(raw, " \t\r\n\x00") {
		return problemf(ErrInvalid, "远程地址包含非法空白或选项前缀")
	}
	if strings.Contains(raw, redactedSecret) {
		return problemf(ErrInvalid, "远程地址包含已脱敏占位符 ***，请输入完整的新地址或留空保持原配置不变")
	}
	return nil
}

func remoteExistsInList(list []Remote, name string) bool {
	for _, item := range list {
		if item.Name == name {
			return true
		}
	}
	return false
}

// AddRemote registers a new Git remote while redacting all URLs in responses, errors, and logs.
func (s *Service) AddRemote(ctx context.Context, repositoryID string, req AddRemoteRequest) (RemoteMutationResult, error) {
	name := strings.TrimSpace(req.Name)
	fetchURL := strings.TrimSpace(req.FetchURL)
	pushURL := strings.TrimSpace(req.PushURL)
	reqHash := hashOperationRequest(GitOpRemoteAdd, repositoryID, name, fetchURL, pushURL)
	return runIdempotentOperation(
		ctx, s, repositoryID, req.ClientRequestID, GitOpRemoteAdd, reqHash,
		func(op GitOperation) (RemoteMutationResult, error) {
			return s.replayRemoteMutationOperation(ctx, repositoryID, op)
		},
		func(opID string) (RemoteMutationResult, error) {
			return s.addRemoteOnce(ctx, repositoryID, name, fetchURL, pushURL, req.ClientRequestID, opID, reqHash)
		},
	)
}

func (s *Service) addRemoteOnce(ctx context.Context, repositoryID, name, fetchURL, pushURL, clientRequestID, opID, reqHash string) (RemoteMutationResult, error) {
	if err := validateRemoteURL(fetchURL); err != nil {
		return RemoteMutationResult{}, err
	}
	if pushURL != "" {
		if err := validateRemoteURL(pushURL); err != nil {
			return RemoteMutationResult{}, err
		}
	}
	if s.gitPath == "" {
		return RemoteMutationResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return RemoteMutationResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return RemoteMutationResult{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(wCtx, r); err != nil {
		return RemoteMutationResult{}, err
	}
	anchor := r.entry.AnchorPath
	if err := s.validateRemoteName(wCtx, anchor, name); err != nil {
		return RemoteMutationResult{}, err
	}

	rawBefore, err := s.git(wCtx, anchor, "remote", "-v")
	if err != nil {
		return RemoteMutationResult{}, err
	}
	if remoteExistsInList(parseRemotes(string(rawBefore)), name) {
		return RemoteMutationResult{}, problemf(ErrConflict, "远程 %s 已存在", name)
	}

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    repositoryID,
		ClientRequestID: clientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpRemoteAdd,
		Status:          GitOpRunning,
		StartedAt:       time.Now().UTC(),
		Summary:         fmt.Sprintf("正在添加远程 %s (%s)", name, redactURL(fetchURL)),
		Preview: GitOperationPreview{
			Remote: name,
			Target: redactURL(fetchURL),
		},
	}); err != nil {
		return RemoteMutationResult{}, err
	}

	run := s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-remote-add:"+opID, "remote", "add", "--", name, fetchURL)
	if run.err == nil && pushURL != "" && pushURL != fetchURL {
		run = s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-remote-add:"+opID, "remote", "set-url", "--push", "--", name, pushURL)
	}
	if run.err != nil {
		retErr := classifyGitMutateError("remote add", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = redactLine(retErr.Error())
			op.SanitizedOutput = run.stderr
		})
		return RemoteMutationResult{}, retErr
	}

	return s.finishRemoteMutationLocked(r, opID, fmt.Sprintf("已添加远程 %s (%s)", name, redactURL(fetchURL)), run.stderr)
}

// UpdateRemote updates the fetch and/or push URL of an existing remote with credential redaction.
func (s *Service) UpdateRemote(ctx context.Context, repositoryID, rawName string, req UpdateRemoteRequest) (RemoteMutationResult, error) {
	name := strings.TrimSpace(rawName)
	var fetchTrim, pushTrim string
	if req.FetchURL != nil {
		fetchTrim = strings.TrimSpace(*req.FetchURL)
	}
	if req.PushURL != nil {
		pushTrim = strings.TrimSpace(*req.PushURL)
	}
	reqHash := hashOperationRequest(
		GitOpRemoteUpdate, repositoryID, name,
		fmtBool(req.FetchURL != nil), fetchTrim,
		fmtBool(req.PushURL != nil), pushTrim,
		fmtBool(req.ClearPushURL),
	)
	return runIdempotentOperation(
		ctx, s, repositoryID, req.ClientRequestID, GitOpRemoteUpdate, reqHash,
		func(op GitOperation) (RemoteMutationResult, error) {
			return s.replayRemoteMutationOperation(ctx, repositoryID, op)
		},
		func(opID string) (RemoteMutationResult, error) {
			return s.updateRemoteOnce(ctx, repositoryID, name, req, fetchTrim, pushTrim, opID, reqHash)
		},
	)
}

func (s *Service) updateRemoteOnce(ctx context.Context, repositoryID, name string, req UpdateRemoteRequest, fetchTrim, pushTrim, opID, reqHash string) (RemoteMutationResult, error) {
	if req.FetchURL == nil && req.PushURL == nil && !req.ClearPushURL {
		return RemoteMutationResult{}, problemf(ErrInvalid, "请至少提供一项要更新的远程地址修改")
	}
	if req.FetchURL != nil {
		if err := validateRemoteURL(fetchTrim); err != nil {
			return RemoteMutationResult{}, err
		}
	}
	if req.PushURL != nil && !req.ClearPushURL {
		if err := validateRemoteURL(pushTrim); err != nil {
			return RemoteMutationResult{}, err
		}
	}
	if s.gitPath == "" {
		return RemoteMutationResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return RemoteMutationResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return RemoteMutationResult{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(wCtx, r); err != nil {
		return RemoteMutationResult{}, err
	}
	anchor := r.entry.AnchorPath
	if err := s.validateRemoteName(wCtx, anchor, name); err != nil {
		return RemoteMutationResult{}, err
	}
	rawBefore, err := s.git(wCtx, anchor, "remote", "-v")
	if err != nil {
		return RemoteMutationResult{}, err
	}
	if !remoteExistsInList(parseRemotes(string(rawBefore)), name) {
		return RemoteMutationResult{}, problemf(ErrNotFound, "远程 %s 不存在", name)
	}

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    repositoryID,
		ClientRequestID: req.ClientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpRemoteUpdate,
		Status:          GitOpRunning,
		StartedAt:       time.Now().UTC(),
		Summary:         fmt.Sprintf("正在更新远程 %s 地址配置", name),
		Preview: GitOperationPreview{
			Remote: name,
		},
	}); err != nil {
		return RemoteMutationResult{}, err
	}

	var lastStderr string
	if req.FetchURL != nil {
		run := s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-remote-update:"+opID, "remote", "set-url", "--", name, fetchTrim)
		lastStderr = run.stderr
		if run.err != nil {
			retErr := classifyGitMutateError("remote set-url", run)
			s.finalizeOperation(opID, func(op *GitOperation) {
				op.Status = GitOpFailed
				op.FailureKind = classifyFailureKind(retErr)
				op.Summary = redactLine(retErr.Error())
				op.SanitizedOutput = run.stderr
			})
			return RemoteMutationResult{}, retErr
		}
	}
	if req.ClearPushURL {
		run := s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-remote-update:"+opID, "config", "--unset-all", "remote."+name+".pushurl")
		lastStderr = run.stderr
		if run.err != nil {
			var exit *exec.ExitError
			if !errors.As(run.err, &exit) || exit.ExitCode() != 5 {
				retErr := classifyGitMutateError("config --unset-all pushurl", run)
				s.finalizeOperation(opID, func(op *GitOperation) {
					op.Status = GitOpFailed
					op.FailureKind = classifyFailureKind(retErr)
					op.Summary = redactLine(retErr.Error())
					op.SanitizedOutput = run.stderr
				})
				return RemoteMutationResult{}, retErr
			}
		}
	} else if req.PushURL != nil {
		run := s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-remote-update:"+opID, "remote", "set-url", "--push", "--", name, pushTrim)
		lastStderr = run.stderr
		if run.err != nil {
			retErr := classifyGitMutateError("remote set-url --push", run)
			s.finalizeOperation(opID, func(op *GitOperation) {
				op.Status = GitOpFailed
				op.FailureKind = classifyFailureKind(retErr)
				op.Summary = redactLine(retErr.Error())
				op.SanitizedOutput = run.stderr
			})
			return RemoteMutationResult{}, retErr
		}
	}

	return s.finishRemoteMutationLocked(r, opID, fmt.Sprintf("已更新远程 %s 的地址配置", name), lastStderr)
}

// RemoveRemote removes an existing remote from a registered repository.
func (s *Service) RemoveRemote(ctx context.Context, repositoryID, rawName, clientRequestID string) (RemoteMutationResult, error) {
	name := strings.TrimSpace(rawName)
	reqHash := hashOperationRequest(GitOpRemoteRemove, repositoryID, name)
	return runIdempotentOperation(
		ctx, s, repositoryID, clientRequestID, GitOpRemoteRemove, reqHash,
		func(op GitOperation) (RemoteMutationResult, error) {
			return s.replayRemoteMutationOperation(ctx, repositoryID, op)
		},
		func(opID string) (RemoteMutationResult, error) {
			return s.removeRemoteOnce(ctx, repositoryID, name, clientRequestID, opID, reqHash)
		},
	)
}

func (s *Service) removeRemoteOnce(ctx context.Context, repositoryID, name, clientRequestID, opID, reqHash string) (RemoteMutationResult, error) {
	if s.gitPath == "" {
		return RemoteMutationResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return RemoteMutationResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return RemoteMutationResult{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(wCtx, r); err != nil {
		return RemoteMutationResult{}, err
	}
	anchor := r.entry.AnchorPath
	if err := s.validateRemoteName(wCtx, anchor, name); err != nil {
		return RemoteMutationResult{}, err
	}
	rawBefore, err := s.git(wCtx, anchor, "remote", "-v")
	if err != nil {
		return RemoteMutationResult{}, err
	}
	if !remoteExistsInList(parseRemotes(string(rawBefore)), name) {
		return RemoteMutationResult{}, problemf(ErrNotFound, "远程 %s 不存在", name)
	}

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    repositoryID,
		ClientRequestID: clientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpRemoteRemove,
		Status:          GitOpRunning,
		StartedAt:       time.Now().UTC(),
		Summary:         fmt.Sprintf("正在移除远程 %s", name),
		Preview: GitOperationPreview{
			Remote: name,
		},
	}); err != nil {
		return RemoteMutationResult{}, err
	}

	run := s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-remote-remove:"+opID, "remote", "remove", "--", name)
	if run.err != nil {
		retErr := classifyGitMutateError("remote remove", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = redactLine(retErr.Error())
			op.SanitizedOutput = run.stderr
		})
		return RemoteMutationResult{}, retErr
	}

	return s.finishRemoteMutationLocked(r, opID, fmt.Sprintf("已移除远程 %s", name), run.stderr)
}

func (s *Service) finishRemoteMutationLocked(r *repositoryState, opID, summary, rawStderr string) (RemoteMutationResult, error) {
	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	repo := s.readRepository(refreshCtx, r)
	rawAfter, err := s.git(refreshCtx, r.entry.AnchorPath, "remote", "-v")
	remotes := []Remote{}
	if err == nil {
		remotes = parseRemotes(string(rawAfter))
	}
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.Summary = summary
		op.SanitizedOutput = rawStderr
	})
	return RemoteMutationResult{
		OperationID: opID,
		Remotes:     remotes,
		Repository:  repo,
	}, nil
}
