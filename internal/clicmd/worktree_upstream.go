package clicmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

func validUpstreamPolicy(policy string) bool {
	return policy == "" || policy == "latest" || policy == "local"
}

func worktreeUpstreamChallenge(err error) *contract.WorktreeUpstream {
	var apiErr *localclient.APIError
	var failure contract.WorktreeUpstreamFailure
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict ||
		json.Unmarshal(apiErr.Body, &failure) != nil || failure.Reason != "worktree_upstream_confirmation" ||
		failure.WorktreeUpstream == nil || failure.WorktreeUpstream.ConfirmationID == "" {
		return nil
	}
	return failure.WorktreeUpstream
}

// chooseWorktreeUpstream answers only a current server challenge. An explicit
// policy never bypasses fetching or converts a branch into an unchecked OID.
func (e Env) chooseWorktreeUpstream(check contract.WorktreeUpstream, policy string) (*contract.UpstreamChoice, error) {
	if !validUpstreamPolicy(policy) {
		return nil, errors.New("--upstream must be latest or local")
	}
	if policy == "" {
		if e.Interactive == nil || !e.Interactive() || e.Stdin == nil {
			writeJSON(e.Stderr, contract.WorktreeUpstreamFailure{Error: "worktree upstream choice required", Reason: "worktree_upstream_confirmation", WorktreeUpstream: &check})
			if !check.CanUseUpstream {
				return nil, errors.New("upstream cannot be used safely; repeat with --upstream=local to keep the local branch, or cancel")
			}
			return nil, errors.New("not a terminal: repeat with --upstream=latest or --upstream=local, or cancel")
		}
		_, _ = fmt.Fprintf(e.Stderr, "webmux: branch %s at %s; upstream %s at %s\n", check.Branch, check.LocalHead, check.Upstream, check.UpstreamHead)
		if check.Warning != "" {
			_, _ = fmt.Fprintln(e.Stderr, "webmux:", check.Warning)
		}
		if check.CanUseUpstream {
			_, _ = fmt.Fprint(e.Stderr, "Use upstream latest, keep local, or cancel? [latest/local/cancel]: ")
		} else {
			_, _ = fmt.Fprint(e.Stderr, "Upstream cannot be used safely. Keep local or cancel? [local/cancel]: ")
		}
		if _, err := fmt.Fscanln(e.Stdin, &policy); err != nil {
			return nil, errors.New("cancelled; no worktree was created")
		}
		policy = strings.TrimSpace(policy)
	}
	if policy != "latest" && policy != "local" {
		return nil, errors.New("cancelled; no worktree was created")
	}
	if policy == "latest" && !check.CanUseUpstream {
		return nil, errors.New("upstream cannot be used safely; use --upstream=local to keep the local branch, or cancel")
	}
	return &contract.UpstreamChoice{ConfirmationID: check.ConfirmationID, UseUpstream: policy == "latest"}, nil
}
