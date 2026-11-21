package workspace

import (
	"context"
	"hash"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultWatchInterval = 2 * time.Second
	defaultWatchDebounce = 250 * time.Millisecond
	maxWatchedWorktrees  = 1024
	maxWatchedHeadBytes  = 512
	maxWatchedGitdirSize = 4096
)

// Subscription receives coalesced notifications when registered repositories
// or their Git worktrees change on disk or through the service.
type Subscription struct {
	C chan struct{}
}

// Subscribe opens a coalesced workspace change subscription and ensures the
// demand-driven Git worktree watcher is running while subscribers exist.
func (s *Service) Subscribe() (*Subscription, error) {
	if err := s.requireRegistry(); err != nil {
		return nil, err
	}
	s.watchMu.Lock()
	if s.closed {
		s.watchMu.Unlock()
		return nil, problemf(ErrUnavailable, "仓库服务已关闭")
	}
	if s.watchSubs == nil {
		s.watchSubs = make(map[*Subscription]struct{})
	}
	sub := &Subscription{C: make(chan struct{}, 1)}
	s.watchSubs[sub] = struct{}{}
	if s.watchStop == nil {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		s.watchStop, s.watchDone = cancel, done
		go s.watchLoop(ctx, done)
	}
	s.watchMu.Unlock()
	return sub, nil
}

// Unsubscribe removes sub and stops the background watcher when the last
// subscriber leaves.
func (s *Service) Unsubscribe(sub *Subscription) {
	if sub == nil {
		return
	}
	s.watchMu.Lock()
	delete(s.watchSubs, sub)
	var stop context.CancelFunc
	var done chan struct{}
	if len(s.watchSubs) == 0 && s.watchStop != nil {
		stop, done = s.watchStop, s.watchDone
		s.watchStop, s.watchDone = nil, nil
	}
	s.watchMu.Unlock()
	if stop != nil {
		stop()
		<-done
	}
}

func (s *Service) closeWatcher() {
	s.watchMu.Lock()
	s.closed = true
	stop, done := s.watchStop, s.watchDone
	s.watchStop, s.watchDone = nil, nil
	clear(s.watchSubs)
	s.watchMu.Unlock()
	if stop != nil {
		stop()
		<-done
	}
}

func (s *Service) notifyWorkspaces() {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	for sub := range s.watchSubs {
		select {
		case sub.C <- struct{}{}:
		default:
		}
	}
}

func (s *Service) watchLoop(ctx context.Context, done chan struct{}) {
	defer close(done)
	interval := s.watchInterval
	if interval <= 0 {
		interval = defaultWatchInterval
	}
	debounce := s.watchDebounce
	if debounce < 0 {
		debounce = 0
	}
	baseline := s.watchFingerprints()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := s.watchFingerprints()
			changed, dirtyIDs := diffWatchFingerprints(baseline, current)
			if !changed {
				continue
			}
			// Let multi-step Git worktree checkout or removal settle before
			// connected clients re-read Git status.
			if debounce > 0 {
				timer := time.NewTimer(debounce)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				current = s.watchFingerprints()
				_, dirtyIDs = diffWatchFingerprints(baseline, current)
			}
			baseline = current
			s.markRepositoriesDirty(dirtyIDs)
			s.reassociate(ctx)
			s.notifyWorkspaces()
		}
	}
}

func diffWatchFingerprints(before, after map[string]uint64) (changed bool, dirtyIDs []string) {
	for id, fp := range after {
		if prev, ok := before[id]; !ok || prev != fp {
			changed = true
			dirtyIDs = append(dirtyIDs, id)
		}
	}
	for id := range before {
		if _, ok := after[id]; !ok {
			changed = true
		}
	}
	return changed, dirtyIDs
}

// watchFingerprints hashes each registered repository's on-disk Git worktree
// metadata independently without spawning git subprocesses or holding service
// locks, so changes in one repository only mark that repository dirty.
func (s *Service) watchFingerprints() map[string]uint64 {
	s.mu.Lock()
	if !s.capability.RegistryAvailable {
		s.mu.Unlock()
		return nil
	}
	entries := make([]registration, len(s.entries))
	for i, r := range s.entries {
		entries[i] = r.entry
	}
	s.mu.Unlock()

	out := make(map[string]uint64, len(entries))
	for _, entry := range entries {
		out[entry.ID] = repoFingerprint(entry)
	}
	return out
}

func repoFingerprint(entry registration) uint64 {
	h := fnv.New64a()
	writeToken(h, entry.ID)
	writeToken(h, entry.CommonDir)
	writeToken(h, entry.AnchorPath)
	writeDirState(h, entry.AnchorPath)
	if !writeDirState(h, entry.CommonDir) {
		return h.Sum64()
	}
	writeBoundedFile(h, filepath.Join(entry.CommonDir, "HEAD"), maxWatchedHeadBytes)
	adminRoot := filepath.Join(entry.CommonDir, "worktrees")
	items, err := os.ReadDir(adminRoot)
	if err != nil {
		writeToken(h, "no-worktrees")
		return h.Sum64()
	}
	if len(items) > maxWatchedWorktrees {
		items = items[:maxWatchedWorktrees]
	}
	for _, item := range items {
		writeToken(h, item.Name())
		if !item.IsDir() {
			writeToken(h, "file")
			continue
		}
		adminDir := filepath.Join(adminRoot, item.Name())
		gitdir := writeBoundedFile(h, filepath.Join(adminDir, "gitdir"), maxWatchedGitdirSize)
		if gitFile := strings.TrimSpace(gitdir); gitFile != "" {
			if !filepath.IsAbs(gitFile) {
				gitFile = filepath.Join(adminDir, gitFile)
			}
			writeDirState(h, filepath.Dir(filepath.Clean(gitFile)))
		}
		writeBoundedFile(h, filepath.Join(adminDir, "HEAD"), maxWatchedHeadBytes)
		writeDirOrFileState(h, filepath.Join(adminDir, "locked"))
	}
	return h.Sum64()
}

func writeToken(h hash.Hash64, value string) {
	_, _ = h.Write([]byte(value))
	_, _ = h.Write([]byte{0})
}

func writeDirState(h hash.Hash64, path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		writeToken(h, "missing")
		return false
	}
	if info.IsDir() {
		writeToken(h, "dir")
		return true
	}
	writeToken(h, "nondir")
	return false
}

func writeDirOrFileState(h hash.Hash64, path string) {
	info, err := os.Stat(path)
	if err != nil {
		writeToken(h, "missing")
		return
	}
	if info.IsDir() {
		writeToken(h, "dir")
		return
	}
	writeToken(h, "present")
}

func writeBoundedFile(h hash.Hash64, path string, limit int64) string {
	f, err := os.Open(path)
	if err != nil {
		writeToken(h, "unreadable")
		return ""
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		writeToken(h, "readerr")
		return ""
	}
	_, _ = h.Write(data)
	_, _ = h.Write([]byte{0})
	return string(data)
}
