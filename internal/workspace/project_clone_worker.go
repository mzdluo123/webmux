package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rainchan/webmux/internal/pathutil"
)

func (s *Service) runProjectClone(op projectCloneRecord) {
	select {
	case s.cloneSlots <- struct{}{}:
		defer func() { <-s.cloneSlots }()
	case <-s.cloneContext.Done():
		s.finishProjectClone(op.ID, "interrupted", "服务停止中断了排队，请使用新的 requestId 重新发起", "")
		return
	}
	ctx, cancel := context.WithTimeout(s.cloneContext, projectCloneTimeout)
	defer cancel()
	temporary, err := pathutil.MakeTemporaryDirectory(filepath.Dir(op.Path), ".webmux-clone-"+op.ID+"-")
	if err != nil {
		s.finishProjectClone(op.ID, "failed", "无法创建克隆临时目录，请检查服务用户的目录权限", "")
		return
	}
	// The shared handle owns only the directory it allocated; neither request
	// paths nor loaded ledger paths can be passed to recursive cleanup.
	defer s.cleanupProjectCloneTemporary(op.ID, temporary)

	if err := s.workerProjectCloneChange(op.ID, func(record *projectCloneRecord) { record.TemporaryPath = temporary.Path() }); err != nil {
		return
	}
	destination := filepath.Join(temporary.Path(), "repository")
	env := s.projectCloneGitEnv(ctx, filepath.Dir(op.Path))
	run := runGitProcess(ctx, gitProcess{
		path:   s.gitPath,
		args:   []string{"-C", filepath.Dir(op.Path), "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "protocol.ssh.allow=always", "clone", "--progress", "--", op.URL, destination},
		env:    env,
		stdout: io.Discard,
		// Cloning can produce much more progress than a local Git command. Bound
		// it independently; none of this output is logged or returned to clients.
		stderrLimit: 1 << 20,
	})
	if run.err != nil || ctx.Err() != nil {
		state, message := "failed", "克隆失败，请检查仓库地址、网络与服务端 Git 凭据，使用新的 requestId 重新发起"
		if ctx.Err() != nil {
			state, message = "interrupted", "服务停止中断了克隆，请使用新的 requestId 重新发起"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				message = "网络克隆超过 10 分钟，请使用新的 requestId 重新发起"
			}
		}
		s.finishProjectClone(op.ID, state, message, "")
		return
	}
	if err := s.workerProjectCloneChange(op.ID, func(record *projectCloneRecord) {
		record.State, record.Error = "registering", ""
	}); err != nil {
		return
	}
	// The durable intent precedes rename, so a crash immediately after publication
	// remains eligible for registration retry without another network clone.
	if err := pathutil.PublishDirectory(destination, op.Path); err != nil {
		s.finishProjectClone(op.ID, "failed", "无法发布克隆目录；请检查目标是否被占用、目录权限以及文件系统是否支持原子独占重命名", "")
		return
	}
	s.registerClonedProject(op)
}

func (s *Service) registerClonedProject(op projectCloneRecord) {
	ctx, cancel := context.WithTimeout(s.cloneContext, writeTimeout)
	defer cancel()
	project, _, err := s.openGitProject(ctx, op.Path)
	if err != nil {
		s.finishProjectClone(op.ID, "registration_failed", "仓库已克隆，但项目登记失败；目录已保留，可重试登记", "")
		return
	}
	s.finishProjectClone(op.ID, "succeeded", "", project.ID)
}

func (s *Service) finishProjectClone(id, state, message, projectID string) {
	// Once a worker exits, a failed transition must fail polling closed rather
	// than advertise an operation as running with no owner. Admission-only
	// pre-publication failures remain retryable without freezing the store.
	_ = s.workerProjectCloneChange(id, func(record *projectCloneRecord) {
		record.State, record.Error, record.ProjectID = state, message, projectID
		record.TemporaryPath = ""
	})
}

// Preserve deployment-key wrappers configured by the service user. Only supply
// the default OpenSSH BatchMode when no custom Git SSH transport was selected.
func (s *Service) projectCloneGitEnv(ctx context.Context, parent string) []string {
	env := append(append([]string{}, gitEnv...), "SSH_ASKPASS_REQUIRE=never")
	if os.Getenv("GIT_SSH_COMMAND") != "" || os.Getenv("GIT_SSH") != "" {
		return env
	}
	out, run := s.runGit(ctx, parent, nil, "config", "--get", "core.sshCommand")
	if run.err == nil && len(out.Bytes()) > 0 {
		return env
	}
	return append(env, "GIT_SSH_COMMAND=ssh -oBatchMode=yes")
}

// Worker failures differ from rejected admission: an admitted operation cannot
// remain advertised as active after its worker exits without a durable result.
func (s *Service) workerProjectCloneChange(id string, change func(*projectCloneRecord)) error {
	if err := s.changeProjectClone(id, change); err != nil {
		s.cloneMu.Lock()
		s.cloneError = fmt.Errorf("克隆操作状态无法确认，请重启服务重新读取: %w", err)
		s.cloneMu.Unlock()
		return err
	}
	return nil
}

func (s *Service) cleanupProjectCloneTemporary(id string, temporary *pathutil.TemporaryDirectory) {
	if err := temporary.Remove(); err != nil {
		_ = s.workerProjectCloneChange(id, func(record *projectCloneRecord) {
			if errors.Is(err, pathutil.ErrTemporaryDirectoryUnowned) {
				record.TemporaryPath = ""
				record.Error = "临时目录身份无法确认，已拒绝清理；该目录不再视为本操作所有"
			} else {
				record.TemporaryPath = temporary.Path()
			}
		})
	}
}
