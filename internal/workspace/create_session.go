package workspace

import (
	"context"
	"errors"
	"fmt"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// createPlan holds only the repository-owned worktree preparation.
type createPlan struct {
	branch string
	base   string
	path   string
}

// planWorktree settles a new worktree's branch, base and path: a new
// branch without a name gets wt/<date>-<time>, free as a branch and as a
// directory; a missing base is the repository's default base; a missing
// path is the branch's last segment under the repository's worktrees
// folder.
func (s *Service) planWorktree(ctx context.Context, r *repositoryState, nw *sessioncreate.NewWorktree, plan *createPlan) error {
	if nw.Mode != "new" && nw.Mode != "existing" {
		return problemf(ErrInvalid, "newWorktree.mode 只能是 new 或 existing")
	}
	plan.branch, plan.base, plan.path = nw.Branch, nw.Base, nw.Path
	if nw.Mode == "existing" && plan.branch == "" {
		return problemf(ErrInvalid, "请选择已有本地分支")
	}
	parent := filepath.Join(filepath.Dir(r.entry.AnchorPath), filepath.Base(r.entry.AnchorPath)+".worktrees")
	if nw.Mode == "new" && (plan.branch == "" || plan.base == "") {
		repository, err := s.current(ctx, r)
		if err != nil {
			return err
		}
		branches, err := s.branches(ctx, r, repository)
		if err != nil {
			return err
		}
		if plan.base == "" {
			if branches.DefaultBase != nil {
				plan.base = *branches.DefaultBase
			} else if len(branches.Branches) != 0 || len(branches.Bases) != 0 {
				plan.base = "HEAD"
			}
		}
		if plan.branch == "" {
			taken := make(map[string]bool, len(branches.Branches))
			for _, b := range branches.Branches {
				taken[b.Name] = true
			}
			plan.branch = generatedBranch(time.Now(), func(branch string) bool {
				if taken[branch] {
					return false
				}
				_, err := os.Lstat(filepath.Join(parent, path.Base(branch)))
				return errors.Is(err, os.ErrNotExist)
			})
		}
	}
	leaf := path.Base(plan.branch)
	if leaf == "." || leaf == "/" || strings.HasPrefix(leaf, "-") {
		return problemf(ErrInvalid, "分支名无效")
	}
	if plan.path == "" {
		plan.path = filepath.Join(parent, leaf)
	}
	return nil
}

// generatedBranch is wt/<date>-<time> at now, with a -2, -3, ... suffix
// until free accepts it.
func generatedBranch(now time.Time, free func(string) bool) string {
	stamp := now.Format("20060102-150405")
	for suffix := 1; suffix <= 1000; suffix++ {
		name := "wt/" + stamp
		if suffix > 1 {
			name = fmt.Sprintf("wt/%s-%d", stamp, suffix)
		}
		if free(name) {
			return name
		}
	}
	return fmt.Sprintf("wt/%s-%d", stamp, now.Nanosecond())
}
