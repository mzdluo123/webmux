package workspace

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/rainchan/webmux/internal/statefile"
)

const maxProjectClones = 500

var cloneRequestID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,119}$`)
var cloneOperationID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var cloneSCPHost = regexp.MustCompile(`^([a-zA-Z0-9_][a-zA-Z0-9_.-]*@)?[a-zA-Z0-9][a-zA-Z0-9.-]*$`)

type projectCloneRecord struct {
	ID            string `json:"id"`
	RequestID     string `json:"requestId"`
	URL           string `json:"url"`
	Path          string `json:"path"`
	TemporaryPath string `json:"temporaryPath"`
	State         string `json:"state"`
	Error         string `json:"error"`
	ProjectID     string `json:"projectId"`
}
type projectCloneRegistry struct {
	Version    int                  `json:"version"`
	Operations []projectCloneRecord `json:"operations"`
}

// validateProjectURL forbids interactive/credential-bearing and local/ext
// transports. Git still receives the accepted value as one argv after --.
func validateProjectURL(raw string) error {
	if raw == "" || len(raw) > 4096 || strings.IndexFunc(raw, unicode.IsControl) >= 0 || strings.HasPrefix(raw, "-") {
		return problemf(ErrInvalid, "请输入有效的 Git 远程地址")
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "ssh") || parsed.Hostname() == "" || parsed.Path == "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || strings.IndexFunc(parsed.Path, unicode.IsControl) >= 0 {
			return problemf(ErrInvalid, "只支持不含凭据或查询参数的 HTTPS / SSH Git 地址")
		}
		if parsed.User != nil {
			_, password := parsed.User.Password()
			if password || parsed.Scheme != "ssh" || strings.IndexFunc(parsed.User.Username(), unicode.IsControl) >= 0 {
				return problemf(ErrInvalid, "不要在地址中填写凭据，请配置服务端 Git 认证")
			}
		}
		return nil
	}
	host, path, ok := strings.Cut(raw, ":")
	if !ok || !cloneSCPHost.MatchString(host) || path == "" || strings.HasPrefix(path, "-") || strings.HasPrefix(path, ":") || strings.ContainsAny(path, "?#") {
		return problemf(ErrInvalid, "请输入 HTTPS、SSH 或 user@host:path 格式的 Git 地址")
	}
	switch strings.ToLower(host) {
	case "ext", "file", "http", "https", "ssh", "git", "ftp", "ftps", "rsync":
		return problemf(ErrInvalid, "不支持此 Git 传输协议")
	}
	return nil
}

func validateProjectClones(stored projectCloneRegistry) error {
	if stored.Version != 1 || stored.Operations == nil || len(stored.Operations) > maxProjectClones {
		return errors.New("不支持或损坏的项目克隆清单")
	}
	ids, requests := map[string]bool{}, map[string]bool{}
	for _, op := range stored.Operations {
		if !cloneOperationID.MatchString(op.ID) || ids[op.ID] || requests[op.RequestID] || !cloneRequestID.MatchString(op.RequestID) || !filepath.IsAbs(op.Path) || filepath.Clean(op.Path) != op.Path || strings.IndexFunc(op.Path, unicode.IsControl) >= 0 || validateProjectURL(op.URL) != nil {
			return errors.New("项目克隆身份或来源无效")
		}
		ids[op.ID], requests[op.RequestID] = true, true
		switch op.State {
		case "running", "registering", "failed", "interrupted", "registration_failed":
		case "succeeded":
			if op.ProjectID == "" {
				return errors.New("项目克隆结果缺少项目标识")
			}
		default:
			return errors.New("项目克隆状态无效")
		}
		if op.TemporaryPath != "" && (!filepath.IsAbs(op.TemporaryPath) || filepath.Dir(op.TemporaryPath) != filepath.Dir(op.Path) || !strings.HasPrefix(filepath.Base(op.TemporaryPath), ".webmux-clone-"+op.ID+"-")) {
			return errors.New("项目克隆临时目录无效")
		}
	}
	return nil
}

func (s *Service) loadProjectClones() {
	var stored projectCloneRegistry
	var err error
	s.cloneStore, stored, err = statefile.Open[projectCloneRegistry](s.stateDir, statefile.ProjectClones, validateProjectClones)
	if statefile.IsKind(err, statefile.Missing) {
		s.clones = []projectCloneRecord{}
		return
	}
	if err != nil {
		s.cloneError = fmt.Errorf("打开项目克隆清单: %w", err)
		return
	}
	changed := false
	for i := range stored.Operations {
		op := &stored.Operations[i]
		switch op.State {
		case "running":
			op.State = "interrupted"
			op.Error = "服务重启中断了克隆，请重新发起；已有临时目录已保留"
			changed = true
		case "registering":
			op.State = "registration_failed"
			op.Error = "克隆已完成，发布或登记被服务重启中断；目标存在时可重试登记，否则重新发起"
			changed = true
		}
	}
	if changed {
		if err := s.cloneStore.Commit(stored); err != nil {
			s.cloneError = fmt.Errorf("恢复项目克隆状态: %w", err)
			return
		}
	}
	s.clones = stored.Operations
}

func (s *Service) closeProjectClones() {
	s.cloneSaveMu.Lock()
	defer s.cloneSaveMu.Unlock()
	s.cloneMu.Lock()
	defer s.cloneMu.Unlock()
	s.cloneStore.Close()
	s.cloneError = errors.New("项目克隆服务已关闭")
}

// ProjectCloneStorageError is independent of the ordinary directory catalog.
func (s *Service) ProjectCloneStorageError() error {
	if s == nil {
		return statefile.ErrUnavailableStore()
	}
	s.cloneMu.Lock()
	defer s.cloneMu.Unlock()
	return s.cloneError
}

// commitProjectClones requires cloneSaveMu. Memory becomes visible only after
// durable publication; only uncertain publication freezes the ledger.
func (s *Service) commitProjectClones(next []projectCloneRecord) error {
	if err := s.cloneStore.Commit(projectCloneRegistry{Version: 1, Operations: next}); err != nil {
		if statefile.IsKind(err, statefile.CommitUncertain) {
			s.cloneMu.Lock()
			s.cloneError = fmt.Errorf("项目克隆清单提交结果不确定，请重启重新读取: %w", err)
			s.cloneMu.Unlock()
			return fmt.Errorf("%w: 项目克隆状态无法保存", ErrUnavailable)
		}
		return fmt.Errorf("保存项目克隆清单: %w", err)
	}
	s.cloneMu.Lock()
	s.clones = slices.Clone(next)
	s.cloneMu.Unlock()
	return nil
}
