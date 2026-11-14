package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/rainchan/webmux/internal/pathutil"
)

const projectCloneTimeout = 10 * time.Minute

// Two network clones may run at once; the durable ledger bounds queued work.
const projectCloneParallelism = 2

type ProjectCloneRequest struct{ RequestID, URL, Path, Source string }
type ProjectCloneOperation struct{ ID, State, Path, Error, ProjectID string }

func cloneOperation(op projectCloneRecord) ProjectCloneOperation {
	return ProjectCloneOperation{ID: op.ID, State: op.State, Path: op.Path, Error: op.Error, ProjectID: op.ProjectID}
}

// StartProjectClone durably admits an operation before it can outlive its HTTP
// request. A request ID always refers to the same source and destination.
func (s *Service) StartProjectClone(ctx context.Context, req ProjectCloneRequest) (ProjectCloneOperation, error) {
	if !cloneRequestID.MatchString(req.RequestID) {
		return ProjectCloneOperation{}, problemf(ErrInvalid, "克隆请求标识无效")
	}
	if err := validateProjectURL(req.URL); err != nil {
		return ProjectCloneOperation{}, err
	}
	if req.Source != "" && req.Source != "github" {
		return ProjectCloneOperation{}, problemf(ErrInvalid, "克隆来源无效")
	}
	if req.Source == "github" {
		if err := validateGitHubCloneURL(req.URL); err != nil {
			return ProjectCloneOperation{}, err
		}
		if err := s.requireGitHubClone(ctx); err != nil {
			return ProjectCloneOperation{}, err
		}
	}
	target, err := projectCloneTarget(req.Path)
	if err != nil {
		return ProjectCloneOperation{}, err
	}
	s.cloneSaveMu.Lock()
	defer s.cloneSaveMu.Unlock()
	if err := s.requireProjectClone(); err != nil {
		return ProjectCloneOperation{}, err
	}
	s.cloneMu.Lock()
	next := slices.Clone(s.clones)
	s.cloneMu.Unlock()
	for _, op := range next {
		if op.RequestID == req.RequestID {
			if op.URL != req.URL || op.Path != target || op.Source != req.Source {
				return ProjectCloneOperation{}, problemf(ErrConflict, "该请求标识已用于其他克隆，请新建请求")
			}
			return cloneOperation(op), nil
		}
	}
	if len(next) >= maxProjectClones {
		return ProjectCloneOperation{}, problemf(ErrInvalid, "克隆操作清单已满")
	}
	if err := ctx.Err(); err != nil {
		return ProjectCloneOperation{}, err
	}
	if _, err := os.Lstat(target); err == nil {
		return ProjectCloneOperation{}, problemf(ErrConflict, "目标已存在，不会覆盖，请选择本地文件夹或修改目录")
	} else if !os.IsNotExist(err) {
		return ProjectCloneOperation{}, fmt.Errorf("%w: 无法检查克隆目录", ErrInvalid)
	}
	id, err := randomID()
	if err != nil {
		return ProjectCloneOperation{}, fmt.Errorf("生成克隆标识: %w", err)
	}
	op := projectCloneRecord{ID: id, RequestID: req.RequestID, URL: req.URL, Path: target, Source: req.Source, State: "running"}
	next = append(next, op)
	if !s.cloneStore.Fits(projectCloneRegistry{Version: 1, Operations: next}) {
		return ProjectCloneOperation{}, problemf(ErrTooLarge, "克隆操作清单超过存储限制")
	}
	if err := s.commitProjectClones(next); err != nil {
		return ProjectCloneOperation{}, err
	}
	s.cloneWG.Go(func() { s.runProjectClone(op) })
	return cloneOperation(op), nil
}

func projectCloneTarget(path string) (string, error) {
	if path == "" || strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return "", problemf(ErrInvalid, "请输入有效的克隆目录")
	}
	expanded, err := pathutil.Expand(path)
	if err != nil {
		return "", fmt.Errorf("%w: 克隆目录无效", ErrInvalid)
	}
	if _, err := pathutil.CleanName(filepath.Base(expanded)); err != nil {
		return "", problemf(ErrInvalid, "克隆目录名称无效")
	}
	parent, err := canonical(filepath.Dir(expanded))
	if err != nil {
		return "", fmt.Errorf("%w: 克隆父目录不可访问", ErrInvalid)
	}
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return "", problemf(ErrInvalid, "克隆父目录必须是已有文件夹")
	}
	return filepath.Join(parent, filepath.Base(expanded)), nil
}

// requireProjectClone is called with cloneSaveMu, including admission's stopped
// check, so no WaitGroup.Go can race shutdown's Wait.
func (s *Service) requireProjectClone() error {
	if s.cloneStopped {
		return problemf(ErrUnavailable, "项目克隆正在停止")
	}
	if s.ProjectStorageError() != nil || s.ProjectCloneStorageError() != nil || s.StorageError() != nil {
		return problemf(ErrUnavailable, "项目克隆存储不可用")
	}
	if s.gitPath == "" {
		return problemf(ErrUnavailable, "Git 不可用，无法克隆")
	}
	return nil
}

func (s *Service) ProjectClone(id string) (ProjectCloneOperation, error) {
	s.cloneMu.Lock()
	defer s.cloneMu.Unlock()
	if s.cloneError != nil {
		return ProjectCloneOperation{}, problemf(ErrUnavailable, "项目克隆状态不可读取")
	}
	for _, op := range s.clones {
		if op.ID == id {
			return cloneOperation(op), nil
		}
	}
	return ProjectCloneOperation{}, problemf(ErrNotFound, "克隆操作不存在")
}

func (s *Service) changeProjectClone(id string, change func(*projectCloneRecord)) error {
	s.cloneSaveMu.Lock()
	defer s.cloneSaveMu.Unlock()
	s.cloneMu.Lock()
	if s.cloneError != nil {
		s.cloneMu.Unlock()
		return problemf(ErrUnavailable, "项目克隆状态不可保存")
	}
	next := slices.Clone(s.clones)
	s.cloneMu.Unlock()
	for i := range next {
		if next[i].ID == id {
			change(&next[i])
			return s.commitProjectClones(next)
		}
	}
	return problemf(ErrNotFound, "克隆操作不存在")
}

// RetryProjectCloneRegistration never clones again. It participates in the
// same detached, joined worker lifetime as the original clone.
func (s *Service) RetryProjectCloneRegistration(ctx context.Context, id string) (ProjectCloneOperation, error) {
	s.cloneSaveMu.Lock()
	defer s.cloneSaveMu.Unlock()
	if err := s.requireProjectClone(); err != nil {
		return ProjectCloneOperation{}, err
	}
	if err := ctx.Err(); err != nil {
		return ProjectCloneOperation{}, err
	}
	s.cloneMu.Lock()
	next := slices.Clone(s.clones)
	s.cloneMu.Unlock()
	for i := range next {
		op := &next[i]
		if op.ID != id {
			continue
		}
		if op.State == "succeeded" || op.State == "registering" {
			return cloneOperation(*op), nil
		}
		if op.State != "registration_failed" {
			return ProjectCloneOperation{}, problemf(ErrConflict, "该克隆没有可重试的登记步骤")
		}
		op.State, op.Error = "registering", ""
		if err := s.commitProjectClones(next); err != nil {
			return ProjectCloneOperation{}, err
		}
		record := *op
		s.cloneWG.Go(func() { s.registerClonedProject(record) })
		return cloneOperation(record), nil
	}
	return ProjectCloneOperation{}, problemf(ErrNotFound, "克隆操作不存在")
}

// StopProjectClones cancels process groups and joins all producers before the
// project catalog, session manager or registry stores are released.
func (s *Service) StopProjectClones() {
	s.cloneSaveMu.Lock()
	s.cloneStopped = true
	if s.cloneCancel != nil {
		s.cloneCancel()
	}
	s.cloneSaveMu.Unlock()
	s.cloneWG.Wait()
}
