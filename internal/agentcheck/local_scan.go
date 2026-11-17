package agentcheck

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/agentcatalog"
	"github.com/rainchan/webmux/internal/agentprofile"
)

const (
	lookupOutputLimit  = 64 << 10
	versionOutputLimit = 4 << 10
	versionConcurrency = 4
	startupPathKey     = "__WEBMUX_PATH__"
)

type localSnapshot struct {
	checkedAt time.Time
	shell     string
	tools     []localTool
}

type localTool struct {
	entry            agentcatalog.Entry
	installed        bool
	executable       string
	path             string
	installedVersion string
	belowVerified    bool
	connector        *localConnector
}

type localConnector struct {
	entry            agentcatalog.ConnectorEntry
	installed        bool
	path             string
	installedVersion string
	belowVerified    bool
}

func (s *Service) scanHost(ctx context.Context, allowPartialVersions bool) (localSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return localSnapshot{}, err
	}
	env := s.baseEnv()
	shell := agentprofile.DefaultShell(s.shellName(env))

	candidates := collectCandidates(s.entries)
	resolved, probeEnv := s.resolveExecutables(ctx, shell, env, candidates)
	if err := ctx.Err(); err != nil {
		return localSnapshot{}, err
	}

	uniquePaths := uniqueResolvedPaths(resolved)
	versions := s.probeVersions(ctx, probeEnv, uniquePaths)
	if err := ctx.Err(); err != nil && !allowPartialVersions {
		return localSnapshot{}, err
	}

	tools := make([]localTool, 0, len(s.entries))
	for _, entry := range s.entries {
		lt := localTool{entry: entry}
		if len(entry.Executables) > 0 {
			lt.executable = entry.Executables[0]
		}
		for _, candidate := range entry.Executables {
			if p := resolved[candidate]; p != "" {
				lt.installed = true
				lt.executable = candidate
				lt.path = p
				lt.installedVersion = versions[p]
				lt.belowVerified = isBelowVerified(lt.installedVersion, entry.VerifiedVersion)
				break
			}
		}
		if entry.Connector != nil {
			ce := *entry.Connector
			lc := &localConnector{entry: ce}
			if p := resolved[ce.Executable]; p != "" {
				lc.installed = true
				lc.path = p
				lc.installedVersion = versions[p]
				lc.belowVerified = isBelowVerified(lc.installedVersion, ce.VerifiedVersion)
			}
			lt.connector = lc
		}
		tools = append(tools, lt)
	}

	effectiveShell := shell
	if effectiveShell == "" {
		effectiveShell = "direct"
	}
	return localSnapshot{
		checkedAt: s.now(),
		shell:     effectiveShell,
		tools:     tools,
	}, nil
}

func isBelowVerified(installed, verified string) bool {
	if installed == "" || verified == "" {
		return false
	}
	order, ok := agentprofile.CompareVersions(installed, verified)
	return ok && order < 0
}

func collectCandidates(entries []agentcatalog.Entry) []string {
	var names []string
	seen := make(map[string]bool, len(entries)*2)
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, e := range entries {
		for _, exe := range e.Executables {
			add(exe)
		}
		if e.Connector != nil {
			add(e.Connector.Executable)
		}
	}
	return names
}

func uniqueResolvedPaths(resolved map[string]string) []string {
	var paths []string
	seen := make(map[string]bool, len(resolved))
	for _, p := range resolved {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

func (s *Service) resolveExecutables(ctx context.Context, shell string, env, names []string) (map[string]string, []string) {
	out := make(map[string]string, len(names))
	if len(names) == 0 {
		return out, env
	}
	if agentprofile.SupportedStartupShell(shell) {
		if shellMap, shellPath, ok := s.lookupInStartupShell(ctx, shell, env, names); ok {
			probeEnv := env
			if shellPath != "" {
				probeEnv = setEnvVar(env, "PATH", shellPath)
			}
			return shellMap, probeEnv
		}
	}
	for _, name := range names {
		if p := lookPathInEnv(name, env); p != "" {
			out[name] = p
		}
	}
	return out, env
}

func (s *Service) lookupInStartupShell(parent context.Context, shell string, env, names []string) (map[string]string, string, bool) {
	shellPath := lookPathInEnv(shell, env)
	if shellPath == "" {
		return nil, "", false
	}
	lookupExpr := `type -P -- "$n" 2>/dev/null`
	if shell == agentprofile.ShellZsh {
		lookupExpr = `whence -p -- "$n" 2>/dev/null`
	}
	script := `printf '\n` + agentprofile.LookupMarker + startupPathKey + `=%s' "$PATH"; for n in "$@"; do printf '\n` + agentprofile.LookupMarker + `%s=%s' "$n" "$(` + lookupExpr + `)"; done; printf '\n'`
	args := append([]string{"-ic", script, "webmux"}, names...)

	ctx, cancel := context.WithTimeout(parent, s.lookupTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shellPath, args...)
	configureCommand(cmd)
	cmd.Stdin = nil
	cmd.Env = env
	buf := &limitedBuffer{limit: lookupOutputLimit}
	cmd.Stdout = buf
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		return nil, "", false
	}
	return parseStartupLookup(buf.Bytes(), names)
}

func parseStartupLookup(raw []byte, names []string) (map[string]string, string, bool) {
	resolved := make(map[string]string, len(names))
	var shellPath string
	count := 0
	for _, line := range bytes.Split(raw, []byte("\n")) {
		rest, ok := bytes.CutPrefix(line, []byte(agentprofile.LookupMarker))
		if !ok {
			continue
		}
		name, pathBytes, ok := bytes.Cut(rest, []byte("="))
		if !ok {
			continue
		}
		if string(name) == startupPathKey {
			shellPath = strings.TrimSpace(string(pathBytes))
			continue
		}
		count++
		p := strings.TrimSpace(string(pathBytes))
		if filepath.IsAbs(p) && isExecutableFile(p) {
			resolved[string(name)] = p
		}
	}
	if count < len(names) {
		return nil, "", false
	}
	return resolved, shellPath, true
}

func (s *Service) probeVersions(parent context.Context, baseEnv, paths []string) map[string]string {
	results := make(map[string]string, len(paths))
	if len(paths) == 0 {
		return results
	}
	env := make([]string, 0, len(baseEnv)+2)
	env = append(env, baseEnv...)
	env = append(env, "CI=1", "NO_COLOR=1")

	sem := make(chan struct{}, versionConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, absPath := range paths {
		if parent.Err() != nil {
			break
		}
		wg.Add(1)
		go func(binary string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-parent.Done():
				return
			}
			defer func() { <-sem }()

			ver := s.readBinaryVersion(parent, env, binary)
			if ver != "" {
				mu.Lock()
				results[binary] = ver
				mu.Unlock()
			}
		}(absPath)
	}
	wg.Wait()
	return results
}

func (s *Service) readBinaryVersion(parent context.Context, env []string, binary string) string {
	ctx, cancel := context.WithTimeout(parent, s.versionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "--version")
	configureCommand(cmd)
	cmd.Stdin = nil
	cmd.Env = env
	buf := &limitedBuffer{limit: versionOutputLimit}
	cmd.Stdout = buf
	cmd.Stderr = buf

	if err := cmd.Run(); err != nil {
		return ""
	}
	return agentprofile.ExtractVersion(buf.String())
}

func lookPathInEnv(file string, env []string) string {
	if strings.ContainsRune(file, filepath.Separator) {
		if filepath.IsAbs(file) && isExecutableFile(file) {
			return file
		}
		return ""
	}
	pathVal := envLookup(env, "PATH")
	for _, dir := range filepath.SplitList(pathVal) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		candidate := filepath.Join(dir, file)
		if isExecutableFile(candidate) {
			return candidate
		}
	}
	return ""
}

func envLookup(env []string, key string) string {
	val := ""
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			val = v
		}
	}
	return val
}

func setEnvVar(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok && k == key {
			if !replaced {
				out = append(out, key+"="+value)
				replaced = true
			}
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, key+"="+value)
	}
	return out
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

type limitedBuffer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	orig := len(p)
	rem := b.limit - b.buf.Len()
	if rem > 0 {
		if len(p) > rem {
			p = p[:rem]
		}
		_, _ = b.buf.Write(p)
	}
	return orig, nil
}

func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.buf.Bytes())
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
