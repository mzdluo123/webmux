package workspace

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const githubPageSize = 20

var githubRepositoryName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*/[a-zA-Z0-9_.-]+$`)

type GitHubRepository struct {
	FullName    string
	Description string
	Private     bool
	CloneURL    string
}
type GitHubProjects struct {
	Available    bool
	Reason       string
	Login        string
	Repositories []GitHubRepository
	Page         int
	HasMore      bool
}

// GitHubAvailable checks only executable discovery. Authentication is probed
// when the user explicitly opens this source, never during capability polling.
func (s *Service) GitHubAvailable() bool { return s != nil && s.githubPath != "" }

func validateGitHubCloneURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || !githubRepositoryName.MatchString(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")) {
		return problemf(ErrInvalid, "GitHub 来源只支持 github.com 的 HTTPS 仓库地址")
	}
	return validateProjectURL(raw)
}

// GitHubProjects returns readable source degradation without leaking gh output.
// It uses the service user's existing gh authentication and never starts login.
func (s *Service) GitHubProjects(ctx context.Context, query string, page int) (GitHubProjects, error) {
	result := GitHubProjects{Repositories: []GitHubRepository{}, Page: page}
	if page < 1 || page > 50 || len(query) > 200 || strings.IndexFunc(query, unicode.IsControl) >= 0 {
		return result, problemf(ErrInvalid, "GitHub 搜索或页码无效")
	}
	if !s.GitHubAvailable() {
		result.Reason = "服务端未安装 GitHub CLI（gh），请使用本地文件夹或 Git URL"
		return result, nil
	}
	s.cloneSaveMu.Lock()
	availability := s.requireProjectClone()
	s.cloneSaveMu.Unlock()
	if availability != nil {
		result.Reason = "服务端 Git 或项目克隆存储不可用，请使用本地文件夹"
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, s.githubTimeout)
	defer cancel()
	if err := s.requireGitHubClone(ctx); err != nil {
		result.Reason = err.Error()
		return result, nil
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := s.githubJSON(ctx, "user", nil, &user); err != nil || user.Login == "" {
		result.Reason = "无法验证服务端 GitHub 身份，请检查服务用户的 gh 登录、权限与网络"
		return result, nil
	}
	result.Login = user.Login
	type repository struct {
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
		CloneURL    string `json:"clone_url"`
	}
	var repos []repository
	fields := []string{"per_page=" + strconv.Itoa(githubPageSize), "page=" + strconv.Itoa(page)}
	query = strings.TrimSpace(query)
	if query == "" {
		fields = append(fields, "affiliation=owner,collaborator,organization_member", "sort=updated")
		if err := s.githubJSON(ctx, "user/repos", fields, &repos); err != nil {
			result.Reason = "无法读取 GitHub 仓库，请检查访问权限、速率限制与网络"
			return result, nil
		}
	} else {
		var search struct {
			Items      []repository `json:"items"`
			Incomplete bool         `json:"incomplete_results"`
		}
		fields = append(fields, "q="+query+" in:name", "sort=updated")
		if err := s.githubJSON(ctx, "search/repositories", fields, &search); err != nil || search.Incomplete {
			result.Reason = "无法完成 GitHub 搜索，请检查访问权限、速率限制与网络后重试"
			return result, nil
		}
		repos = search.Items
	}
	for _, repo := range repos {
		if !githubRepositoryName.MatchString(repo.FullName) || validateGitHubCloneURL(repo.CloneURL) != nil {
			result.Reason = "GitHub 返回了无法识别的仓库数据，请重试"
			result.Repositories = []GitHubRepository{}
			return result, nil
		}
		result.Repositories = append(result.Repositories, GitHubRepository(repo))
	}
	result.Available = true
	result.HasMore = len(repos) == githubPageSize && page < 50
	return result, nil
}

func (s *Service) githubJSON(ctx context.Context, endpoint string, fields []string, target any) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := &boundedOutput{limit: 4 << 20, cancel: cancel}
	args := []string{"api", "--hostname", "github.com", "--method", "GET", endpoint}
	for _, field := range fields {
		args = append(args, "--raw-field", field)
	}
	run := runGitProcess(ctx, gitProcess{path: s.githubPath, args: args, env: []string{"GH_PROMPT_DISABLED=1", "GH_PAGER=cat", "GH_DEBUG=", "GH_HOST=github.com"}, stdout: out, stderrLimit: 64 << 10})
	if run.err != nil {
		return problemf(ErrUnavailable, "GitHub 请求失败")
	}
	if err := json.Unmarshal(out.Bytes(), target); err != nil {
		return problemf(ErrUnavailable, "GitHub 响应无效")
	}
	return nil
}

const githubRequestTimeout = 20 * time.Second

// Probe the local command grammar only on explicit source use/admission. Older
// distro packages cannot suppress fork-parent fetches; fail before downloading
// instead of blaming credentials after a successful private-fork clone.
func (s *Service) requireGitHubClone(ctx context.Context) error {
	if !s.GitHubAvailable() {
		return problemf(ErrUnavailable, "服务端未安装 GitHub CLI（gh）")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out := &boundedOutput{limit: 64 << 10, cancel: cancel}
	run := runGitProcess(ctx, gitProcess{path: s.githubPath, args: []string{"repo", "clone", "--help"}, env: []string{"GH_PROMPT_DISABLED=1", "GH_DEBUG=", "GH_NO_UPDATE_NOTIFIER=1"}, stdout: out, stderrLimit: 64 << 10})
	if run.err != nil {
		return problemf(ErrUnavailable, "无法检查服务端 gh 克隆能力，请检查 CLI 安装与执行权限")
	}
	if !strings.Contains(out.String(), "--no-upstream") {
		return problemf(ErrUnavailable, "GitHub 来源需要支持 --no-upstream 的 gh（2.88 或更新版本）；请升级服务端 gh，或使用 Git URL 来源")
	}
	return nil
}
