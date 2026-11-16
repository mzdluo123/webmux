package agentcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/agentprofile"
)

const (
	defaultUpstreamTimeout = 6 * time.Second
	maxUpstreamBodyBytes   = 64 << 10
	maxRedirects           = 3
	upstreamConcurrency    = 4
)

var defaultNPMBases = []string{
	"https://registry.npmjs.org",
	"https://registry.npmmirror.com",
}

const (
	defaultPyPIBase      = "https://pypi.org"
	defaultGitHubAPIBase = "https://api.github.com"
)

type remoteEntry struct {
	version   string
	err       string
	checkedAt time.Time
}

func newHardenedClient(base *http.Client, timeout time.Duration, allowed map[string]string) *http.Client {
	var transport http.RoundTripper
	if base != nil && base.Transport != nil {
		transport = base.Transport
	} else {
		transport = http.DefaultTransport
	}
	if base != nil && base.Timeout > 0 && base.Timeout < timeout {
		timeout = base.Timeout
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("重定向次数过多")
			}
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			host := strings.ToLower(req.URL.Host)
			wantScheme, ok := allowed[host]
			if !ok || req.URL.Scheme != wantScheme {
				return fmt.Errorf("拒绝重定向至不受信任地址 %s://%s", req.URL.Scheme, req.URL.Host)
			}
			return nil
		},
	}
}

func buildAllowedHosts(bases ...string) map[string]string {
	allowed := make(map[string]string, len(bases))
	for _, raw := range bases {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			continue
		}
		allowed[strings.ToLower(u.Host)] = u.Scheme
	}
	return allowed
}

func collectInstalledUpstreams(snap localSnapshot) []agentprofile.UpstreamSource {
	var out []agentprofile.UpstreamSource
	seen := make(map[agentprofile.UpstreamSource]bool)
	add := func(u agentprofile.UpstreamSource) {
		if u.Kind == agentprofile.UpstreamNone || strings.TrimSpace(u.Package) == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	for _, t := range snap.tools {
		if t.installed {
			add(t.entry.Distribution.Upstream)
		}
		if t.connector != nil && t.connector.installed {
			add(t.connector.entry.Distribution.Upstream)
		}
	}
	return out
}

func (s *Service) fetchUpstreams(ctx context.Context, sources []agentprofile.UpstreamSource) map[agentprofile.UpstreamSource]remoteEntry {
	now := s.now()
	results := make(map[agentprofile.UpstreamSource]remoteEntry, len(sources))
	if len(sources) == 0 {
		return results
	}

	sem := make(chan struct{}, upstreamConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			mu.Lock()
			results[src] = remoteEntry{
				checkedAt: now,
				err:       summarizeUpstreamError(err),
			}
			mu.Unlock()
			continue
		}
		wg.Add(1)
		go func(u agentprofile.UpstreamSource) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				mu.Lock()
				results[u] = remoteEntry{
					checkedAt: now,
					err:       summarizeUpstreamError(ctx.Err()),
				}
				mu.Unlock()
				return
			}
			defer func() { <-sem }()

			ver, err := s.queryOneUpstream(ctx, u)
			entry := remoteEntry{checkedAt: now}
			if err != nil {
				entry.err = summarizeUpstreamError(err)
			} else {
				entry.version = ver
			}
			mu.Lock()
			results[u] = entry
			mu.Unlock()
		}(src)
	}
	wg.Wait()
	return results
}

func (s *Service) queryOneUpstream(ctx context.Context, u agentprofile.UpstreamSource) (string, error) {
	switch u.Kind {
	case agentprofile.UpstreamNPM:
		return s.queryNPM(ctx, u.Package)
	case agentprofile.UpstreamPyPI:
		return s.queryPyPI(ctx, u.Package)
	case agentprofile.UpstreamGitHubRelease:
		return s.queryGitHubRelease(ctx, u.Package)
	default:
		return "", fmt.Errorf("未知上游类型 %q", u.Kind)
	}
}

func (s *Service) queryNPM(ctx context.Context, pkg string) (string, error) {
	path, err := npmLatestPath(pkg)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, base := range s.npmBases {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		endpoint := strings.TrimRight(base, "/") + path
		body, err := s.getJSON(ctx, endpoint, "")
		if err != nil {
			lastErr = err
			continue
		}
		var payload struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			lastErr = fmt.Errorf("解析 npm 响应失败: %w", err)
			continue
		}
		ver := agentprofile.ExtractVersion(payload.Version)
		if ver == "" {
			lastErr = errors.New("npm 响应缺少有效版本号")
			continue
		}
		return ver, nil
	}
	if lastErr == nil {
		lastErr = errors.New("未配置 npm 源")
	}
	return "", lastErr
}

func npmLatestPath(pkg string) (string, error) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" || strings.Contains(pkg, "..") {
		return "", errors.New("无效的 npm 包名")
	}
	if strings.HasPrefix(pkg, "@") {
		scope, name, ok := strings.Cut(pkg[1:], "/")
		if !ok || scope == "" || name == "" || strings.Contains(name, "/") {
			return "", errors.New("无效的 npm 作用域包名")
		}
		return "/@" + url.PathEscape(scope) + "/" + url.PathEscape(name) + "/latest", nil
	}
	if strings.Contains(pkg, "/") {
		return "", errors.New("无效的 npm 包名")
	}
	return "/" + url.PathEscape(pkg) + "/latest", nil
}

func (s *Service) queryPyPI(ctx context.Context, pkg string) (string, error) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" || strings.Contains(pkg, "/") || strings.Contains(pkg, "..") {
		return "", errors.New("无效的 PyPI 包名")
	}
	endpoint := strings.TrimRight(s.pypiBase, "/") + "/pypi/" + url.PathEscape(pkg) + "/json"
	body, err := s.getJSON(ctx, endpoint, "")
	if err != nil {
		return "", err
	}
	var payload struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("解析 PyPI 响应失败: %w", err)
	}
	ver := agentprofile.ExtractVersion(payload.Info.Version)
	if ver == "" {
		return "", errors.New("PyPI 响应缺少有效版本号")
	}
	return ver, nil
}

func (s *Service) queryGitHubRelease(ctx context.Context, repo string) (string, error) {
	owner, name, ok := strings.Cut(strings.TrimSpace(repo), "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") || strings.Contains(repo, "..") {
		return "", errors.New("无效的 GitHub 仓库标识")
	}
	endpoint := strings.TrimRight(s.githubAPIBase, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/releases/latest"
	body, err := s.getJSON(ctx, endpoint, "application/vnd.github+json")
	if err != nil {
		return "", err
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("解析 GitHub Release 响应失败: %w", err)
	}
	ver := agentprofile.ExtractVersion(payload.TagName)
	if ver == "" {
		return "", errors.New("GitHub Release 响应缺少有效版本号")
	}
	return ver, nil
}

func (s *Service) getJSON(ctx context.Context, endpoint, accept string) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	wantScheme, ok := s.allowedHosts[strings.ToLower(u.Host)]
	if !ok || u.Scheme != wantScheme {
		return nil, fmt.Errorf("拒绝访问不受信任地址 %s://%s", u.Scheme, u.Host)
	}
	reqCtx, cancel := context.WithTimeout(ctx, s.client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("User-Agent", "WebMux-AgentCheck/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("上游返回 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxUpstreamBodyBytes {
		return nil, errors.New("上游响应体积超出上限")
	}
	return body, nil
}

func summarizeUpstreamError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 160 {
		msg = msg[:160] + "..."
	}
	return msg
}
