package workspace

import (
	"context"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/sessioncreate"
)

type UpstreamChoice = sessioncreate.UpstreamChoice

type WorktreeUpstream = sessioncreate.WorktreeUpstream
type UpstreamConfirmationError = sessioncreate.UpstreamConfirmationError

type upstreamSource struct {
	branch, ref, head, upstream, upstreamRef, upstreamHead, remote, merge string
}

type upstreamConfirmation struct {
	repositoryID, mode, branch, base string
	source                           upstreamSource
	canUse                           bool
	expires                          time.Time
}

type upstreamCheck struct {
	source      upstreamSource
	base        string
	useUpstream bool
	checked     bool
}

// creationSource runs with the repository gate held. Only a branch or a commit
// that still names a branch tip participates; historical detached commits remain
// exact, as do release/rebuild operations (which do not call CreateWorktree).
func (s *Service) creationSource(ctx context.Context, r *repositoryState, req CreateRequest) (upstreamSource, error) {
	var source upstreamSource
	if req.Mode == "orphan" || (req.Mode != "new" && req.Mode != "existing") {
		return source, nil
	}
	base := req.Base
	if req.Mode == "existing" {
		base = "refs/heads/" + req.Branch
	}
	if base == "" {
		base = "HEAD"
	}
	if strings.HasPrefix(base, "-") || strings.ContainsRune(base, '\x00') {
		return source, problemf(ErrInvalid, "基线无效")
	}
	head, err := s.git(ctx, r.entry.AnchorPath, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	if err != nil {
		if ctx.Err() != nil {
			return source, err
		}
		return source, nil // normal creation validation diagnoses unborn/invalid bases
	}
	oid := outputLine(head)
	if strings.ContainsAny(base, "~^") {
		return source, nil
	}
	data, err := s.git(ctx, r.entry.AnchorPath, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(upstream:short)%00%(upstream)", "refs/heads/", "refs/remotes/")
	if err != nil {
		return source, err
	}
	resolved, _ := s.git(ctx, r.entry.AnchorPath, "rev-parse", "--symbolic-full-name", "--verify", "--end-of-options", base)
	ref := outputLine(resolved)
	var candidates []upstreamSource
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		parts := strings.Split(line, "\x00")
		if len(parts) != 4 || parts[1] != oid || strings.HasSuffix(parts[0], "/HEAD") {
			continue
		}
		candidate := upstreamSource{ref: parts[0], head: oid, upstream: parts[2], upstreamRef: parts[3]}
		candidate.branch = strings.TrimPrefix(strings.TrimPrefix(candidate.ref, "refs/heads/"), "refs/remotes/")
		if candidate.ref == ref {
			source = candidate
			break
		}
		if strings.HasPrefix(candidate.ref, "refs/remotes/") {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if source.ref == "" {
		// Prefer the checked-out branch when an OID is shared by several refs.
		current, _ := s.git(ctx, r.entry.AnchorPath, "symbolic-ref", "-q", "HEAD")
		for _, candidate := range candidates {
			if candidate.ref == outputLine(current) {
				source = candidate
				break
			}
		}
		if source.ref == "" && len(candidates) == 1 {
			source = candidates[0]
		}
	}
	if source.ref == "" {
		return source, nil
	}
	if strings.HasPrefix(source.ref, "refs/remotes/") {
		source.remote = s.configuredRemote(ctx, r.entry.AnchorPath, source.branch)
		source.upstream, source.upstreamRef, source.upstreamHead = source.branch, source.ref, source.head
		return source, nil
	}
	if source.upstreamRef == "" {
		return source, nil
	}
	remote, err := s.git(ctx, r.entry.AnchorPath, "config", "--get", "branch."+source.branch+".remote")
	if err != nil {
		return source, err
	}
	merge, err := s.git(ctx, r.entry.AnchorPath, "config", "--get", "branch."+source.branch+".merge")
	if err != nil {
		return source, err
	}
	source.remote, source.merge = outputLine(remote), outputLine(merge)
	if source.remote == "." || !strings.HasPrefix(source.upstreamRef, "refs/remotes/") {
		source.remote = ""
		return source, nil
	}
	upstream, err := s.git(ctx, r.entry.AnchorPath, "rev-parse", "--verify", "--end-of-options", source.upstreamRef+"^{commit}")
	if err == nil {
		source.upstreamHead = outputLine(upstream)
	} else if ctx.Err() != nil {
		return source, err
	}
	return source, nil
}

func (s *Service) readCreationSource(ctx context.Context, r *repositoryState, req CreateRequest) (upstreamSource, error) {
	if err := acquire(ctx, r); err != nil {
		return upstreamSource{}, err
	}
	defer release(r)
	return s.creationSource(ctx, r, req)
}

func (s *Service) upstreamChallenge(repositoryID string, req CreateRequest, source upstreamSource, warning string, canUse bool) error {
	id, err := randomID()
	if err != nil {
		return err
	}
	now := time.Now()
	s.mu.Lock()
	if s.upstreamConfirmations == nil {
		s.upstreamConfirmations = make(map[string]upstreamConfirmation)
	}
	for key, confirmation := range s.upstreamConfirmations {
		if now.After(confirmation.expires) {
			delete(s.upstreamConfirmations, key)
		}
	}
	s.upstreamConfirmations[id] = upstreamConfirmation{repositoryID: repositoryID, mode: req.Mode, branch: req.Branch, base: req.Base, source: source, canUse: canUse, expires: now.Add(confirmationLifetime)}
	s.mu.Unlock()
	return &UpstreamConfirmationError{WorktreeUpstream: WorktreeUpstream{ConfirmationID: id, Branch: source.branch, LocalHead: source.head, Upstream: source.upstream, UpstreamHead: source.upstreamHead, Warning: warning, CanUseUpstream: canUse}}
}

// checkCreationUpstream never holds the repository gate over a network fetch.
// The resulting snapshot is checked again under the mutation gate immediately
// before worktree add. RefreshBranchBase's existing fallback behavior is intact.
func (s *Service) checkCreationUpstream(ctx context.Context, repositoryID string, req CreateRequest) (upstreamCheck, error) {
	check := upstreamCheck{base: req.Base}
	r, err := s.repository(repositoryID)
	if err != nil {
		return check, err
	}
	if req.LatestUpstream && (req.Mode != "new" || req.UpstreamChoice != nil) {
		return check, problemf(ErrInvalid, "自动选择上游仅适用于从基线新建分支")
	}
	sourceReq := req
	var previous upstreamConfirmation
	if req.UpstreamChoice != nil {
		s.mu.Lock()
		previous = s.upstreamConfirmations[req.UpstreamChoice.ConfirmationID]
		s.mu.Unlock()
		if previous.repositoryID == repositoryID && previous.mode == req.Mode && previous.base == req.Base && req.Mode == "new" {
			sourceReq.Base = previous.source.ref
		}
	}
	before, err := s.readCreationSource(ctx, r, sourceReq)
	if err != nil {
		return check, err
	}
	if strings.HasPrefix(before.ref, "refs/remotes/") && previous.source.ref == before.ref {
		before.head = previous.source.head
	}
	check.source = before
	if req.Mode == "new" && before.ref != "" {
		check.base = before.head
	}
	if before.ref == "" || before.remote == "" {
		if req.UpstreamChoice != nil {
			if before.ref != "" && !req.UpstreamChoice.UseUpstream && previous.repositoryID == repositoryID && previous.mode == req.Mode && previous.branch == req.Branch && previous.base == req.Base && previous.source == before && time.Now().Before(previous.expires) {
				check.checked = true
				return check, nil
			}
			if before.ref == "" {
				before = previous.source
			}
			return check, s.upstreamChallenge(repositoryID, req, before, "上游确认已失效，请重新选择基线", false)
		}
		return check, nil
	}
	// When the user already confirmed keeping the local commit and the local
	// source ref/config has not changed since that confirmation, skip a second
	// network fetch (which would also stall again when offline).
	if choice := req.UpstreamChoice; choice != nil && !choice.UseUpstream {
		if time.Now().Before(previous.expires) &&
			previous.repositoryID == repositoryID &&
			previous.mode == req.Mode &&
			previous.branch == req.Branch &&
			previous.base == req.Base &&
			previous.source == before {
			check.checked = true
			check.useUpstream = false
			if req.Mode == "new" {
				check.base = before.head
			}
			return check, nil
		}
	}
	refreshed, err := s.RefreshBranchBase(ctx, repositoryID, before.branch)
	if err != nil {
		return check, err
	}
	if err := ctx.Err(); err != nil {
		return check, err
	}
	if req.Mode == "new" {
		sourceReq.Base = before.ref
	}
	after, err := s.readCreationSource(ctx, r, sourceReq)
	if err != nil {
		return check, err
	}
	check.source, check.checked = after, true
	// A remote-tracking source's own tip is expected to advance during fetch;
	// preserve the pre-fetch local choice while binding the fetched tip.
	remoteOnly := strings.HasPrefix(before.ref, "refs/remotes/")
	if remoteOnly && after.ref == before.ref {
		after.head = before.head
		check.source = after
	}
	moved := before.ref != after.ref || before.remote != after.remote || before.merge != after.merge || before.upstreamRef != after.upstreamRef || !remoteOnly && before.head != after.head
	if req.LatestUpstream {
		if refreshed.UpstreamHead == "" || remoteOnly && refreshed.FetchWarning != "" {
			return check, problemf(ErrInvalid, "无法获取最新上游提交，请重试")
		}
		if moved || refreshed.Upstream != after.upstream || refreshed.UpstreamHead != after.upstreamHead {
			return check, problemf(ErrInvalid, "刷新期间上游已变化，请重试")
		}
		check.base = refreshed.UpstreamHead
		return check, nil
	}
	warning := ""
	canUse, needs := false, false
	if refreshed.UpstreamHead == "" || strings.Contains(refreshed.FetchWarning, "失败") || strings.Contains(refreshed.FetchWarning, "无法") || strings.Contains(refreshed.FetchWarning, "不存在") {
		warning, needs = refreshed.FetchWarning, true
		if warning == "" {
			warning = "无法确认最新上游，请确认是否使用本地提交"
		}
	} else {
		upstreamHead := after.upstreamHead
		if remoteOnly {
			upstreamHead = refreshed.UpstreamHead
			after.upstreamHead = upstreamHead
			check.source = after
		}
		if upstreamHead != "" && upstreamHead != after.head {
			_, containsErr := s.git(ctx, r.entry.AnchorPath, "merge-base", "--is-ancestor", upstreamHead, after.head)
			if ctx.Err() != nil {
				return check, ctx.Err()
			}
			needs = containsErr != nil
			if needs {
				_, ffErr := s.git(ctx, r.entry.AnchorPath, "merge-base", "--is-ancestor", after.head, upstreamHead)
				if ctx.Err() != nil {
					return check, ctx.Err()
				}
				canUse = req.Mode == "new" || ffErr == nil
				warning = "上游包含本地尚未包含的提交，是否从最新上游创建工作树？"
				if ffErr != nil {
					warning = "本地与上游已分叉；保留本地提交，或选择上游创建新分支"
					if req.Mode == "existing" {
						warning = "本地与上游已分叉，不能安全快进已有分支；可保留本地或取消"
					}
				}
			}
		}
	}
	// Config/source movement during fetch always requires a fresh confirmation.
	if moved {
		needs = true
		canUse = false
		warning = "检查期间源分支发生变化，请重新确认本地基线"
	}
	if choice := req.UpstreamChoice; choice != nil {
		s.mu.Lock()
		confirmation, ok := s.upstreamConfirmations[choice.ConfirmationID]
		s.mu.Unlock()
		valid := ok && time.Now().Before(confirmation.expires) && confirmation.repositoryID == repositoryID && confirmation.mode == req.Mode && confirmation.branch == req.Branch && confirmation.base == req.Base && confirmation.source == after
		if !valid {
			return check, s.upstreamChallenge(repositoryID, req, after, "上游确认已失效，请重新确认。"+warning, canUse)
		}
		if choice.UseUpstream && (!canUse || !confirmation.canUse) {
			return check, s.upstreamChallenge(repositoryID, req, after, warning, canUse)
		}
		check.useUpstream = choice.UseUpstream
	} else if needs {
		return check, s.upstreamChallenge(repositoryID, req, after, warning, canUse)
	}
	if req.Mode == "new" {
		check.base = after.head
		if check.useUpstream {
			check.base = after.upstreamHead
		}
	}
	return check, nil
}

func (s *Service) validateCreationUpstream(ctx context.Context, r *repositoryState, req CreateRequest, check upstreamCheck) error {
	if !check.checked {
		return ctx.Err()
	}
	sourceReq := req
	if req.Mode == "new" {
		sourceReq.Base = check.source.ref
	}
	actual, err := s.creationSource(ctx, r, sourceReq)
	if err != nil {
		return err
	}
	if strings.HasPrefix(check.source.ref, "refs/remotes/") && actual.ref == check.source.ref {
		actual.head = check.source.head
	}
	if actual != check.source {
		if req.LatestUpstream {
			return problemf(ErrInvalid, "源分支或上游在创建前发生变化，请重试")
		}
		return s.upstreamChallenge(r.entry.ID, req, actual, "源分支或上游在创建前发生变化，请重新确认", false)
	}
	return ctx.Err()
}
