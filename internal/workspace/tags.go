package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CreateTagRequest creates a local lightweight or annotated tag on a verified commit OID.
type CreateTagRequest struct {
	Name            string `json:"name"`
	TargetOID       string `json:"targetOid"`
	Annotated       bool   `json:"annotated"`
	Message         string `json:"message,omitempty"`
	ClientRequestID string `json:"clientRequestId,omitempty"`
}

// CreateTagResult returns the newly created tag and updated repository branch/tag catalog.
type CreateTagResult struct {
	OperationID string   `json:"operationId,omitempty"`
	Tag         Tag      `json:"tag"`
	Branches    Branches `json:"branches"`
}

// DeleteTagRequest deletes a local tag without touching remote tags.
type DeleteTagRequest struct {
	Name              string `json:"name"`
	ExpectedTargetOID string `json:"expectedTargetOid,omitempty"`
	ClientRequestID   string `json:"clientRequestId,omitempty"`
}

// DeleteTagResult returns the deleted tag target OID, recovery command, and refreshed catalog.
type DeleteTagResult struct {
	OperationID     string   `json:"operationId,omitempty"`
	Name            string   `json:"name"`
	TargetOID       string   `json:"targetOid"`
	RecoveryCommand string   `json:"recoveryCommand"`
	Branches        Branches `json:"branches"`
}

func (s *Service) validTagName(ctx context.Context, anchor, name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsRune(name, 0) || strings.ContainsAny(name, " \t\r\n") {
		return problemf(ErrInvalid, "标签名称无效")
	}
	if _, err := s.git(ctx, anchor, "check-ref-format", "refs/tags/"+name); err != nil {
		if errorKind(err) != ErrInternal {
			return err
		}
		return problemf(ErrInvalid, "标签名称不符合 Git 引用格式规则")
	}
	return nil
}

func isTruthyConfig(val string) bool {
	v := strings.TrimSpace(strings.ToLower(val))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

// CreateTag creates a local lightweight or annotated tag on a verified commit without `-f`.
func (s *Service) CreateTag(ctx context.Context, repositoryID string, req CreateTagRequest) (CreateTagResult, error) {
	name := strings.TrimSpace(req.Name)
	targetOID := strings.TrimSpace(req.TargetOID)
	reqHash := hashOperationRequest(GitOpTagCreate, repositoryID, name, targetOID, fmtBool(req.Annotated), req.Message)
	return runIdempotentOperation(
		ctx, s, repositoryID, req.ClientRequestID, GitOpTagCreate, reqHash,
		func(op GitOperation) (CreateTagResult, error) {
			return s.replayTagCreateOperation(ctx, repositoryID, op)
		},
		func(opID string) (CreateTagResult, error) {
			return s.createTagOnce(ctx, repositoryID, name, targetOID, req.Annotated, req.Message, req.ClientRequestID, opID, reqHash)
		},
	)
}

func (s *Service) createTagOnce(ctx context.Context, repositoryID, name, targetOID string, annotated bool, message, clientRequestID, opID, reqHash string) (CreateTagResult, error) {
	if !isObjectID(targetOID) {
		return CreateTagResult{}, problemf(ErrInvalid, "targetOid 必须是完整的十六进制提交 OID")
	}
	if annotated && strings.TrimSpace(message) == "" {
		return CreateTagResult{}, problemf(ErrInvalid, "创建附注标签必须填写标签说明")
	}
	if s.gitPath == "" {
		return CreateTagResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}

	r, err := s.repository(repositoryID)
	if err != nil {
		return CreateTagResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return CreateTagResult{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(wCtx, r); err != nil {
		return CreateTagResult{}, err
	}
	anchor := r.entry.AnchorPath
	if err := s.validTagName(wCtx, anchor, name); err != nil {
		return CreateTagResult{}, err
	}
	if _, err := s.git(wCtx, anchor, "rev-parse", "--verify", "--end-of-options", targetOID+"^{commit}"); err != nil {
		return CreateTagResult{}, problemf(ErrNotFound, "目标提交 %s 不存在", shortOID(targetOID))
	}
	if existing, eErr := s.resolveCommitReadOnly(wCtx, anchor, "refs/tags/"+name+"^{commit}"); eErr == nil && existing != "" {
		return CreateTagResult{}, problemf(ErrConflict, "本地标签 %s 已存在（指向 %s），拒绝强制覆盖现有标签", name, shortOID(existing))
	}

	tagGpgSign, _, _ := s.configValue(wCtx, anchor, "tag.gpgSign")
	forceSignAnnotated, _, _ := s.configValue(wCtx, anchor, "tag.forceSignAnnotated")
	if !annotated && isTruthyConfig(tagGpgSign) {
		return CreateTagResult{}, problemf(ErrBlocked, "当前仓库已启用 tag.gpgSign 强制签名策略，轻量标签不支持签名，请切换为附注标签并提供说明")
	}
	if annotated {
		ident, iErr := s.readCommitIdentity(wCtx, anchor)
		if iErr != nil {
			return CreateTagResult{}, iErr
		}
		if strings.TrimSpace(ident.CommitterName) == "" || strings.TrimSpace(ident.CommitterEmail) == "" {
			return CreateTagResult{}, problemf(ErrBlocked, "缺少 Git user.name 或 user.email 身份配置，无法创建附注标签")
		}
	}

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    repositoryID,
		ClientRequestID: clientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpTagCreate,
		Status:          GitOpRunning,
		StartedAt:       time.Now().UTC(),
		Summary:         fmt.Sprintf("正在创建标签 %s", name),
		Preview: GitOperationPreview{
			Target:    name,
			TargetOID: targetOID,
		},
	}); err != nil {
		return CreateTagResult{}, err
	}

	tagCtx, tagCancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	stdout := &boundedOutput{limit: maxWorkspaceGitOutput, cancel: tagCancel}
	args := []string{"-C", anchor, "-c", "user.useConfigOnly=true", "tag"}
	var stdin *strings.Reader
	if annotated {
		if isTruthyConfig(tagGpgSign) || isTruthyConfig(forceSignAnnotated) {
			args = append(args, "-s")
		} else {
			args = append(args, "-a")
		}
		args = append(args, "-F", "-", "--", name, targetOID)
		stdin = strings.NewReader(message)
	} else {
		args = append(args, "--no-sign", "--", name, targetOID)
	}

	env := append([]string{}, gitEnv...)
	env = append(env,
		"GIT_REFLOG_ACTION=webmux-tag-create:"+opID,
		"GIT_EDITOR=true",
		"GPG_TTY=",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"SSH_ASKPASS_REQUIRE=never",
	)
	proc := gitProcess{
		path:        s.gitPath,
		args:        args,
		env:         env,
		stdout:      stdout,
		stderrLimit: maxWorkspaceGitError,
	}
	if stdin != nil {
		proc.stdin = stdin
	}
	run := runGitProcess(tagCtx, proc)
	tagCancel()

	if run.err != nil {
		retErr := classifyTagError("tag create", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return CreateTagResult{}, retErr
	}

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	repo, err := s.current(refreshCtx, r)
	if err != nil {
		repo = Repository{ID: repositoryID}
	}
	branches, err := s.branches(refreshCtx, r, repo)
	if err != nil {
		return CreateTagResult{}, err
	}
	created := Tag{
		Name: name,
		Head: targetOID,
		Kind: "lightweight",
	}
	if annotated {
		created.Kind = "annotated"
		created.Message = strings.TrimSpace(message)
	}
	for _, t := range branches.Tags {
		if t.Name == name {
			created = t
			break
		}
	}
	summary := fmt.Sprintf("已在 %s 上创建标签 %s", shortOID(targetOID), name)
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = targetOID
		op.Subject = created.Subject
		op.Summary = summary
		op.SanitizedOutput = run.stderr
	})
	return CreateTagResult{
		OperationID: opID,
		Tag:         created,
		Branches:    branches,
	}, nil
}

// DeleteTag removes a local tag and returns its pre-GC recovery command (`git tag <name> <oid>`).
func (s *Service) DeleteTag(ctx context.Context, repositoryID string, req DeleteTagRequest) (DeleteTagResult, error) {
	name := strings.TrimSpace(req.Name)
	expectedOID := strings.TrimSpace(req.ExpectedTargetOID)
	reqHash := hashOperationRequest(GitOpTagDelete, repositoryID, name, expectedOID)
	return runIdempotentOperation(
		ctx, s, repositoryID, req.ClientRequestID, GitOpTagDelete, reqHash,
		func(op GitOperation) (DeleteTagResult, error) {
			return s.replayTagDeleteOperation(ctx, repositoryID, op)
		},
		func(opID string) (DeleteTagResult, error) {
			return s.deleteTagOnce(ctx, repositoryID, name, expectedOID, req.ClientRequestID, opID, reqHash)
		},
	)
}

func (s *Service) deleteTagOnce(ctx context.Context, repositoryID, name, expectedOID, clientRequestID, opID, reqHash string) (DeleteTagResult, error) {
	if s.gitPath == "" {
		return DeleteTagResult{}, problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return DeleteTagResult{}, err
	}
	wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
	defer wCancel()
	if err := acquire(wCtx, r); err != nil {
		return DeleteTagResult{}, err
	}
	defer release(r)
	if err := s.verifyIdentity(wCtx, r); err != nil {
		return DeleteTagResult{}, err
	}
	anchor := r.entry.AnchorPath
	if err := s.validTagName(wCtx, anchor, name); err != nil {
		return DeleteTagResult{}, err
	}
	targetOID, err := s.resolveCommitReadOnly(wCtx, anchor, "refs/tags/"+name+"^{commit}")
	if err != nil || targetOID == "" {
		return DeleteTagResult{}, problemf(ErrNotFound, "本地标签 %s 不存在", name)
	}
	if expectedOID != "" && expectedOID != targetOID {
		return DeleteTagResult{}, problemf(ErrStale, "标签 %s 指向已变化（当前指向 %s），请刷新后重试", name, shortOID(targetOID))
	}

	if err := s.recordOperationStart(GitOperation{
		ID:              opID,
		RepositoryID:    repositoryID,
		ClientRequestID: clientRequestID,
		RequestHash:     reqHash,
		Kind:            GitOpTagDelete,
		Status:          GitOpRunning,
		StartedAt:       time.Now().UTC(),
		Summary:         fmt.Sprintf("正在删除本地标签 %s", name),
		Preview: GitOperationPreview{
			Target:    name,
			TargetOID: targetOID,
		},
	}); err != nil {
		return DeleteTagResult{}, err
	}

	run := s.runNonInteractiveMutatingGit(ctx, anchor, "webmux-tag-delete:"+opID, "tag", "-d", "--", name)
	if run.err != nil {
		retErr := classifyGitMutateError("tag -d", run)
		s.finalizeOperation(opID, func(op *GitOperation) {
			op.Status = GitOpFailed
			op.FailureKind = classifyFailureKind(retErr)
			op.Summary = retErr.Error()
			op.SanitizedOutput = run.stderr
		})
		return DeleteTagResult{}, retErr
	}

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	repo, rErr := s.current(refreshCtx, r)
	if rErr != nil {
		repo = Repository{ID: repositoryID}
	}
	branches, bErr := s.branches(refreshCtx, r, repo)
	if bErr != nil {
		branches = Branches{Branches: []Branch{}, Bases: []Base{}, Tags: []Tag{}}
	}
	recovery := fmt.Sprintf("git tag %s %s", name, targetOID)
	summary := fmt.Sprintf("已删除本地标签 %s（原指向 %s）", name, shortOID(targetOID))
	s.finalizeOperation(opID, func(op *GitOperation) {
		op.Status = GitOpSucceeded
		op.ActualOID = targetOID
		op.Summary = summary
		op.SanitizedOutput = run.stderr
	})
	return DeleteTagResult{
		OperationID:     opID,
		Name:            name,
		TargetOID:       targetOID,
		RecoveryCommand: recovery,
		Branches:        branches,
	}, nil
}

func classifyTagError(action string, run gitExit) error {
	if isGitLockBusy(run.stderr) {
		return problemf(ErrLocked, "仓库正被其他进程占用（Git 锁冲突），未删除任何锁文件，请稍后重试")
	}
	var launch *exec.Error
	if errors.As(run.err, &launch) || errors.Is(run.err, os.ErrNotExist) || errors.Is(run.err, os.ErrPermission) {
		return problemf(ErrUnavailable, "Git 无法执行: %v", run.err)
	}
	if run.ctxErr != nil {
		return problemf(ErrTimeout, "Git %s 执行超时: %v", action, run.ctxErr)
	}
	return problemf(ErrHookFailed, "Git 标签签名或写入失败: %s", firstGitLine(redactLine(run.stderr)))
}
