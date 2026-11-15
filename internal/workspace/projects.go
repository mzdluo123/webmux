package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/statefile"
)

// MaxProjects bounds the independent directory catalog. Repository projects
// remain owned by the existing repository registry.
const MaxProjects = 500

// Project is a directory in which a session can start. RepositoryID is an
// optional reference, never a copy of Git state or initialization instructions.
type Project struct {
	ID           string
	Name         string
	Path         string
	RepositoryID string
	Availability string
	Error        string
}

type directoryProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}
type projectRegistry struct {
	Version  int                `json:"version"`
	Projects []directoryProject `json:"projects"`
}

func validateProjects(stored projectRegistry) error {
	if stored.Version != 1 || stored.Projects == nil || len(stored.Projects) > MaxProjects {
		return errors.New("不支持或损坏的项目清单")
	}
	ids, paths := map[string]bool{}, map[string]bool{}
	for _, p := range stored.Projects {
		if !strings.HasPrefix(p.ID, "directory:") || len(p.ID) <= len("directory:") || p.Name == "" || !filepath.IsAbs(p.Path) || filepath.Clean(p.Path) != p.Path || ids[p.ID] || paths[p.Path] {
			return errors.New("项目身份或目录无效或重复")
		}
		ids[p.ID], paths[p.Path] = true, true
	}
	return nil
}

func (s *Service) loadProjects() {
	var stored projectRegistry
	var err error
	s.projectStore, stored, err = statefile.Open[projectRegistry](s.stateDir, statefile.Projects, validateProjects)
	if statefile.IsKind(err, statefile.Missing) {
		s.projects = []directoryProject{}
		return
	}
	if err != nil {
		s.projectError = fmt.Errorf("打开项目清单: %w", err)
		return
	}
	s.projects = stored.Projects
}

func (s *Service) closeProjects() {
	s.projectSaveMu.Lock()
	defer s.projectSaveMu.Unlock()
	s.projectMu.Lock()
	defer s.projectMu.Unlock()
	s.projectStore.Close()
	s.projectError = errors.New("项目服务已关闭")
}

// ProjectStorageError reports project storage availability independently of Git
// and the repository registry. It never probes an external command.
func (s *Service) ProjectStorageError() error {
	if s == nil {
		return statefile.ErrUnavailableStore()
	}
	s.projectMu.Lock()
	defer s.projectMu.Unlock()
	return s.projectError
}

// ListProjects overlays registered repositories on explicit directory records.
// Repository IDs remain stable without rewriting the old registry or opening
// Git. Missing directories remain visible with an unavailable reason.
func (s *Service) ListProjects() ([]Project, error) {
	s.projectMu.Lock()
	if s.projectError != nil {
		err := s.projectError
		s.projectMu.Unlock()
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	directories := slices.Clone(s.projects)
	s.projectMu.Unlock()
	s.mu.Lock()
	repositories := make([]registration, 0, len(s.entries))
	for _, r := range s.entries {
		repositories = append(repositories, r.entry)
	}
	s.mu.Unlock()
	result := make([]Project, 0, len(directories)+len(repositories))
	byPath := make(map[string]int)
	for _, p := range directories {
		byPath[p.Path] = len(result)
		result = append(result, Project{ID: p.ID, Name: p.Name, Path: p.Path})
	}
	for _, r := range repositories {
		if i, ok := byPath[r.AnchorPath]; ok {
			result[i].RepositoryID = r.ID
			continue
		}
		byPath[r.AnchorPath] = len(result)
		result = append(result, Project{ID: "repository:" + r.ID, Name: r.DisplayName, Path: r.AnchorPath, RepositoryID: r.ID})
	}
	for i := range result {
		p := &result[i]
		info, err := os.Stat(p.Path)
		p.Availability = "available"
		if err != nil || !info.IsDir() {
			p.Availability = "unavailable"
			p.Error = "项目目录不可访问或已不是文件夹"
		}
	}
	slices.SortFunc(result, func(a, b Project) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return result, nil
}

// RegisterDirectoryProject records an existing directory without init, clone,
// or a Git subprocess. Git discovery is a separate, explicit operation.
// A failed durable commit never publishes an in-memory project.
func (s *Service) RegisterDirectoryProject(path string) (Project, bool, error) {
	if path == "" {
		return Project{}, false, problemf(ErrInvalid, "请选择项目目录")
	}
	expanded, err := pathutil.Expand(path)
	if err != nil {
		return Project{}, false, fmt.Errorf("%w: 项目目录: %w", ErrInvalid, err)
	}
	canonical, err := filepath.EvalSymlinks(expanded)
	if err != nil {
		return Project{}, false, fmt.Errorf("%w: 项目目录: %w", ErrInvalid, err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return Project{}, false, fmt.Errorf("%w: 项目目录: %w", ErrInvalid, err)
	}
	if !info.IsDir() {
		return Project{}, false, problemf(ErrInvalid, "项目路径必须是文件夹")
	}
	s.projectSaveMu.Lock()
	defer s.projectSaveMu.Unlock()
	projects, err := s.ListProjects()
	if err != nil {
		return Project{}, false, err
	}
	for _, p := range projects {
		if p.Path == canonical {
			return p, false, nil
		}
	}
	s.projectMu.Lock()
	next := slices.Clone(s.projects)
	s.projectMu.Unlock()
	if len(next) >= MaxProjects {
		return Project{}, false, problemf(ErrInvalid, "最多登记 %d 个项目", MaxProjects)
	}
	id, err := randomID()
	if err != nil {
		return Project{}, false, fmt.Errorf("生成项目标识: %w", err)
	}
	record := directoryProject{ID: "directory:" + id, Name: filepath.Base(canonical), Path: canonical}
	next = append(next, record)
	if err := s.projectStore.Commit(projectRegistry{Version: 1, Projects: next}); err != nil {
		if statefile.IsKind(err, statefile.CommitUncertain) {
			s.projectMu.Lock()
			s.projectError = fmt.Errorf("项目清单提交结果不确定，请重启重新读取: %w", err)
			s.projectMu.Unlock()
		}
		return Project{}, false, fmt.Errorf("保存项目清单: %w", err)
	}
	s.projectMu.Lock()
	s.projects = next
	s.projectMu.Unlock()
	return Project{ID: record.ID, Name: record.Name, Path: record.Path, Availability: "available"}, true, nil
}
