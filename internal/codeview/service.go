package codeview

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

type binding struct {
	Context
	identity       os.FileInfo
	gitDirs        []string
	gitIdentity    os.FileInfo
	commonIdentity os.FileInfo
	commonDir      string
	timeUsed       time.Time
}
type snapshot struct {
	owner             scopeKey
	context           string
	file              *File
	diff              *Diff
	size              int
	created, timeUsed time.Time
}

// The roots a session can browse: its workspace (launch directory or the
// Git worktree containing it) and its interaction artifacts folder.
const (
	RootWorkspace = "workspace"
	RootArtifacts = "artifacts"
)

type scopeKind uint8

const (
	scopeSession scopeKind = iota
	scopeRepository
	scopeWorktree
)

type scopeKey struct {
	kind    scopeKind
	owner   string
	subroot string
}

func sessionScope(id, root string) scopeKey {
	return scopeKey{kind: scopeSession, owner: id, subroot: root}
}

func targetScope(t Target) scopeKey {
	if t.WorktreeID == "" {
		return scopeKey{kind: scopeRepository, owner: t.RepositoryID}
	}
	return scopeKey{kind: scopeWorktree, owner: t.RepositoryID + "\x00" + t.WorktreeID}
}

func ownerScope(k scopeKey) scopeKey {
	return scopeKey{kind: k.kind, owner: k.owner}
}

type Service struct {
	manager   *session.Manager
	artifacts *artifacts.Service
	workspace TargetResolver
	mu        sync.Mutex
	roots     map[scopeKey]*binding
	snapshots map[string]*snapshot
	bytes     int
	slots     chan struct{}
	transfers chan struct{}
	done      chan struct{}
	once      sync.Once
}

// Limit concurrent Git/blob materialization to two jobs and raw downloads to
// four transfers, bounding CPU, open descriptors and per-request memory.
const (
	materializationConcurrency = 2
	transferConcurrency        = 4
)

// New browses sessions of m. A nil folders leaves the artifacts root
// unavailable.
func New(m *session.Manager, folders *artifacts.Service) *Service {
	s := &Service{manager: m, artifacts: folders, roots: map[scopeKey]*binding{}, snapshots: map[string]*snapshot{}, slots: make(chan struct{}, materializationConcurrency), transfers: make(chan struct{}, transferConcurrency), done: make(chan struct{})}
	go s.sweep()
	return s
}

// UseWorkspace installs, once at assembly before serving, the read-only
// resolver for session-independent repository and worktree contexts.
func (s *Service) UseWorkspace(r TargetResolver) {
	s.mu.Lock()
	s.workspace = r
	s.mu.Unlock()
}

func (s *Service) Close() {
	s.once.Do(func() {
		close(s.done)
		s.mu.Lock()
		clear(s.roots)
		clear(s.snapshots)
		s.bytes = 0
		s.mu.Unlock()
	})
}
func (s *Service) sweep() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var managerDone <-chan struct{}
	if s.manager != nil {
		managerDone = s.manager.Done()
	}
	for {
		select {
		case <-s.done:
			return
		case <-managerDone:
			s.Close()
			return
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for key, b := range s.roots {
				if key.kind == scopeSession {
					if s.manager == nil {
						delete(s.roots, key)
					} else if _, err := s.manager.Get(key.owner); err != nil {
						delete(s.roots, key)
					}
				} else if now.Sub(b.timeUsed) > Lifetime {
					delete(s.roots, key)
				}
			}
			for id, v := range s.snapshots {
				expired := now.Sub(v.created) > Lifetime
				if !expired && v.owner.kind == scopeSession {
					if s.manager == nil {
						expired = true
					} else if _, err := s.manager.Get(v.owner.owner); err != nil {
						expired = true
					}
				}
				if expired {
					s.bytes -= v.size
					delete(s.snapshots, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

// fallbackIDs keeps ids unique if the system random source ever fails.
var fallbackIDs atomic.Uint64

func random() string {
	var b [18]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := fallbackIDs.Add(1)
		return "fallback-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(n, 36)
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func digest(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func nodeID(p string) string { return base64.RawURLEncoding.EncodeToString([]byte(p)) }
func nodePath(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	p := string(b)
	if err != nil || strings.ContainsRune(p, 0) || filepath.IsAbs(p) {
		return "", problemf(ErrInvalid, "无效文件标识")
	}
	if p != "" {
		for _, part := range strings.Split(p, "/") {
			if part == "" || part == "." || part == ".." || strings.EqualFold(part, ".git") {
				return "", problemf(ErrInvalid, "路径越界或 Git 内部路径")
			}
		}
	}
	return p, nil
}
func (s *Service) acquire(ctx context.Context) (func(), error) {
	select {
	case s.slots <- struct{}{}:
		return func() { <-s.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, problemf(ErrUnavailable, "代码服务已关闭")
	}
}
func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	return workspace.ReadGit(ctx, root, MaxBytes, args...)
}

// bind performs filesystem and Git validation; callers must hold a service slot.
func (s *Service) bind(ctx context.Context, key scopeKey) (*binding, error) {
	switch key.kind {
	case scopeRepository:
		return s.bindRepository(ctx, key)
	case scopeWorktree:
		return s.bindWorktree(ctx, key)
	}
	if key.subroot == RootArtifacts {
		return s.bindArtifacts(key.owner)
	}
	if s.manager == nil {
		return nil, problemf(ErrNotFound, "会话不存在")
	}
	item, err := s.manager.Get(key.owner)
	if err != nil {
		return nil, problemf(ErrNotFound, "会话不存在")
	}
	key = sessionScope(key.owner, RootWorkspace)
	s.mu.Lock()
	b := s.roots[key]
	if b != nil {
		b.timeUsed = time.Now()
	}
	s.mu.Unlock()
	if b != nil {
		return b, s.validate(ctx, b)
	}
	cwd, err := filepath.EvalSymlinks(item.Info().Cwd)
	if err != nil {
		return nil, err
	}
	for _, segment := range strings.Split(cwd, string(os.PathSeparator)) {
		if strings.EqualFold(segment, ".git") {
			return nil, problemf(ErrConflict, "会话启动目录位于 Git 内部，代码浏览不可用")
		}
	}
	if raw, e := git(ctx, cwd, "rev-parse", "--is-inside-git-dir"); e == nil && strings.TrimSpace(string(raw)) == "true" {
		return nil, problemf(ErrConflict, "Git 管理目录不可作为浏览根")
	}
	root := cwd
	b = &binding{Context: Context{ID: random(), Kind: "directory", Browse: true, Reason: "普通目录或 Git 不可用；忽略规则与差异不可用"}, timeUsed: time.Now()}
	if out, e := git(ctx, cwd, "rev-parse", "--show-toplevel"); e == nil {
		root = strings.TrimSuffix(string(out), "\n")
		root, e = filepath.EvalSymlinks(root)
		if e != nil {
			return nil, e
		}
		b.Kind = "worktree"
		b.Diff = true
		b.Ignore = true
		b.Reason = ""
		for _, arg := range []string{"--absolute-git-dir", "--git-common-dir"} {
			out, e = git(ctx, root, "rev-parse", "--path-format=absolute", arg)
			if e != nil {
				return nil, e
			}
			p, e := filepath.EvalSymlinks(strings.TrimSuffix(string(out), "\n"))
			if e != nil {
				return nil, e
			}
			b.gitDirs = append(b.gitDirs, p)
		}
		b.commonDir = b.gitDirs[1]
		b.commonIdentity, err = os.Stat(b.gitDirs[1])
		if err != nil {
			return nil, err
		}
		b.gitIdentity, err = os.Stat(b.gitDirs[0])
		if err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	resolver := s.workspace
	s.mu.Unlock()
	if resolver != nil && b.Diff {
		if linked := resolver.Association(item.Info().ID); linked != nil {
			expected, e := filepath.EvalSymlinks(linked.WorktreePath)
			if e != nil || expected != root {
				return nil, problemf(ErrConflict, "工作区关联与浏览根不一致")
			}
		}
	}
	b.Root = root
	b.identity, err = os.Stat(root)
	if err != nil {
		return nil, err
	}
	b.Write = writable(root)
	// A canceled Git probe must not be cached as a plain-directory fallback.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if old := s.roots[key]; old != nil {
		old.timeUsed = time.Now()
		b = old
	} else {
		s.roots[key] = b
	}
	s.mu.Unlock()
	return b, s.validate(ctx, b)
}

// bindArtifacts binds session id's artifacts folder: a plain directory
// without Git, ignore rules or diffs. A folder replaced since it was bound
// gets a new binding, so requests against the old context conflict.
func (s *Service) bindArtifacts(id string) (*binding, error) {
	if s.manager == nil {
		return nil, problemf(ErrNotFound, "会话不存在")
	}
	if _, err := s.manager.Get(id); err != nil {
		return nil, problemf(ErrNotFound, "会话不存在")
	}
	if s.artifacts == nil {
		return nil, problemf(ErrUnavailable, "临时文件夹不可用")
	}
	dir, err := s.artifacts.Existing(id)
	switch {
	case errors.Is(err, artifacts.ErrNotFound):
		return nil, problemf(ErrNotFound, "临时文件夹尚未创建")
	case err != nil:
		return nil, problemf(ErrUnavailable, "临时文件夹不可用")
	}
	key := sessionScope(id, RootArtifacts)
	s.mu.Lock()
	defer s.mu.Unlock()
	if b := s.roots[key]; b != nil {
		if s.validate(context.Background(), b) == nil {
			b.timeUsed = time.Now()
			return b, nil
		}
		delete(s.roots, key)
	}
	identity, err := os.Lstat(dir)
	if err != nil || !identity.IsDir() {
		return nil, problemf(ErrUnavailable, "临时文件夹不可用")
	}
	b := &binding{Context: Context{ID: random(), Root: dir, Kind: RootArtifacts, Browse: true, Write: writable(dir)}, identity: identity, timeUsed: time.Now()}
	s.roots[key] = b
	return b, nil
}

// Context returns the browsing context of session id's root: RootWorkspace
// (also for an empty root) or RootArtifacts.
func (s *Service) Context(ctx context.Context, id, root string) (Context, error) {
	switch root {
	case "":
		root = RootWorkspace
	case RootWorkspace, RootArtifacts:
	default:
		return Context{}, problemf(ErrInvalid, "未知的浏览根")
	}
	return withScope(ctx, s, sessionScope(id, root), func(b *binding) (Context, error) { return b.Context, nil })
}

// withScope holds a service slot while fn uses the validated binding of key.
// Failing to acquire the slot or bind returns T's zero value.
func withScope[T any](ctx context.Context, s *Service, key scopeKey, fn func(*binding) (T, error)) (T, error) {
	release, err := s.acquire(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	defer release()
	b, err := s.bind(ctx, key)
	if err != nil {
		var zero T
		return zero, err
	}
	return fn(b)
}

// withBinding holds a service slot while fn uses session id's validated
// binding of root. Failing to acquire the slot or bind returns T's zero value.
func withBinding[T any](ctx context.Context, s *Service, id, root string, fn func(*binding) (T, error)) (T, error) {
	return withScope(ctx, s, sessionScope(id, root), fn)
}

// withRoot is withBinding for requests made against browsing context cid; it
// rejects them once the session's context has been replaced. cid selects
// the root: the artifacts folder's current context, else the workspace.
func withRoot[T any](ctx context.Context, s *Service, id, cid string, fn func(*binding) (T, error)) (T, error) {
	s.mu.Lock()
	root := s.rootOfLocked(id, cid)
	s.mu.Unlock()
	return withBinding(ctx, s, id, root, func(b *binding) (T, error) {
		if b.ID != cid {
			var zero T
			return zero, problemf(ErrConflict, "浏览上下文已改变")
		}
		return fn(b)
	})
}

func (s *Service) rootOfLocked(id, cid string) string {
	if b := s.roots[sessionScope(id, RootArtifacts)]; b != nil && b.ID == cid {
		return RootArtifacts
	}
	return RootWorkspace
}

// snapshotRoot is the root snapshot sid was taken from, for re-reading it.
func (s *Service) snapshotRoot(id, sid string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v := s.snapshots[sid]; v != nil && v.owner == ownerScope(sessionScope(id, "")) {
		return s.rootOfLocked(id, v.context)
	}
	return RootWorkspace
}
