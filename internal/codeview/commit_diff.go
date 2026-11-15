package codeview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/workspace"
)

const commitSourcePrefix = "commit:"

func isCommitSource(source string) bool {
	oid, ok := strings.CutPrefix(source, commitSourcePrefix)
	return ok && isFullCommitOID(oid)
}

func isFullCommitOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func (b *binding) resolveFromTreeRef(ctx context.Context, fromOID string) (diffRef, canonicalFrom string, isEmpty bool, err error) {
	cwd := b.gitCwd()
	if cwd == "" {
		return "", "", false, problemf(ErrConflict, "当前上下文无可用 Git 仓库")
	}
	emptyOut, err := git(ctx, cwd, "hash-object", "-t", "tree", os.DevNull)
	if err != nil {
		return "", "", false, err
	}
	emptyOID := strings.TrimSpace(string(emptyOut))
	if fromOID == "" || fromOID == emptyOID {
		return emptyOID, "", true, nil
	}
	if err := b.verifyCommitOID(ctx, fromOID); err != nil {
		return "", "", false, err
	}
	return fromOID, fromOID, false, nil
}

func (b *binding) verifyCommitOID(ctx context.Context, oid string) error {
	if !isFullCommitOID(oid) {
		return problemf(ErrInvalid, "版本标识必须是完整的十六进制提交 OID")
	}
	cwd := b.gitCwd()
	if cwd == "" {
		return problemf(ErrConflict, "当前上下文无可用 Git 仓库")
	}
	out, err := git(ctx, cwd, "rev-parse", "--verify", "--end-of-options", oid+"^{commit}")
	if err != nil {
		if errorKind(err) == ErrTimeout || errorKind(err) == ErrTooLarge {
			return err
		}
		return problemf(ErrNotFound, "提交对象不存在")
	}
	if strings.TrimSpace(string(out)) != oid {
		return problemf(ErrNotFound, "提交对象不存在")
	}
	return nil
}

func (b *binding) hasCommitTransformAttr(ctx context.Context, commitOID, p string) (bool, error) {
	cwd := b.gitCwd()
	if b.Root != "" {
		attrs, err := git(ctx, b.Root, "check-attr", "-z", "--source="+commitOID, "filter", "working-tree-encoding", "--", p)
		if err != nil {
			return false, err
		}
		if hasTransformAttr(attrs) {
			return true, nil
		}
		liveAttrs, liveErr := git(ctx, b.Root, "check-attr", "-z", "filter", "working-tree-encoding", "--", p)
		if liveErr == nil && hasTransformAttr(liveAttrs) {
			return true, nil
		}
		return false, nil
	}
	if attrs, err := git(ctx, cwd, "check-attr", "-z", "--source="+commitOID, "filter", "working-tree-encoding", "--", p); err == nil {
		return hasTransformAttr(attrs), nil
	}
	// Non-bare .git common directories require manual .gitattributes matching when
	// no worktree root is bound.
	return b.treeHasTransformAttr(ctx, cwd, commitOID, p)
}

func (b *binding) treeHasTransformAttr(ctx context.Context, cwd, commitOID, targetPath string) (bool, error) {
	var attrCandidates []string
	dir := ""
	parts := strings.Split(targetPath, "/")
	attrCandidates = append(attrCandidates, ".gitattributes")
	for i := 0; i < len(parts)-1; i++ {
		if dir == "" {
			dir = parts[i]
		} else {
			dir = dir + "/" + parts[i]
		}
		attrCandidates = append(attrCandidates, dir+"/.gitattributes")
	}
	args := append([]string{"ls-tree", "-z", commitOID, "--"}, attrCandidates...)
	raw, err := git(ctx, cwd, args...)
	if err != nil {
		return false, err
	}
	var rules []attrRule
	for _, rec := range strings.Split(string(raw), "\x00") {
		header, attrPath, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") {
			continue
		}
		content, catErr := git(ctx, cwd, "cat-file", "blob", fields[2])
		if catErr != nil {
			continue
		}
		attrDir := path.Dir(attrPath)
		if attrDir == "." {
			attrDir = ""
		}
		rules = append(rules, parseAttrRules(attrDir, string(content))...)
	}
	if infoAttr, readErr := os.ReadFile(filepath.Join(cwd, "info", "attributes")); readErr == nil {
		rules = append(rules, parseAttrRules("", string(infoAttr))...)
	}
	return evalTransformRules(targetPath, rules), nil
}

type attrRule struct {
	dir     string
	pattern string
	tokens  []string
}

func parseAttrRules(dir, content string) []attrRule {
	var out []attrRule
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		out = append(out, attrRule{dir: dir, pattern: fields[0], tokens: fields[1:]})
	}
	return out
}

func evalTransformRules(targetPath string, rules []attrRule) bool {
	active := map[string]bool{}
	baseName := path.Base(targetPath)
	for _, rule := range rules {
		rel := targetPath
		if rule.dir != "" {
			var ok bool
			rel, ok = strings.CutPrefix(targetPath, rule.dir+"/")
			if !ok {
				continue
			}
		}
		matched := false
		if strings.Contains(rule.pattern, "/") {
			m, err := path.Match(strings.TrimPrefix(rule.pattern, "/"), rel)
			matched = err == nil && m
		} else {
			m, err := path.Match(rule.pattern, baseName)
			matched = err == nil && m
		}
		if !matched {
			continue
		}
		for _, tok := range rule.tokens {
			for _, name := range []string{"filter", "working-tree-encoding"} {
				switch {
				case tok == "-"+name || tok == "!"+name:
					active[name] = false
				case tok == name || strings.HasPrefix(tok, name+"="):
					active[name] = true
				}
			}
		}
	}
	return active["filter"] || active["working-tree-encoding"]
}

func (b *binding) scanCommitChanges(ctx context.Context, diffRef, canonicalFrom, toOID string, visit func(Change) error) (string, error) {
	cwd := b.gitCwd()
	hash := sha256.New()
	emit := func(c Change) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.Group = "commit"
		c.NodeID = nodeID(c.NewPath)
		c.ID = version([]string{canonicalFrom, toOID, c.OldPath, c.NewPath, c.Status, c.OldOID, c.NewOID, c.OldMode, c.NewMode})
		c.ReviewVersion = c.ID
		_, _ = io.WriteString(hash, c.ID+"\x00")
		if visit != nil {
			return visit(c)
		}
		return nil
	}
	args := []string{"diff-tree", "-r", "--raw", "-z", "--no-abbrev", "--find-renames", diffRef, toOID, "--"}
	phase := 0
	var c Change
	err := workspace.ReadGitRecords(ctx, cwd, func(record string) error {
		switch phase {
		case 0:
			fields := strings.Fields(record)
			if len(fields) != 5 || !strings.HasPrefix(fields[0], ":") {
				return problemf(ErrInternal, "无效提交差异输出")
			}
			c = Change{
				OldMode: strings.TrimPrefix(fields[0], ":"),
				NewMode: fields[1],
				OldOID:  fields[2],
				NewOID:  fields[3],
				Status:  fields[4],
			}
			phase = 1
		case 1:
			if record == "" {
				return problemf(ErrInternal, "无效提交差异路径")
			}
			c.OldPath, c.NewPath = record, record
			if strings.HasPrefix(c.Status, "R") || strings.HasPrefix(c.Status, "C") {
				phase = 2
			} else {
				phase = 0
				return emit(c)
			}
		case 2:
			if record == "" {
				return problemf(ErrInternal, "无效提交重命名路径")
			}
			c.NewPath = record
			phase = 0
			return emit(c)
		}
		return nil
	}, args...)
	if err != nil {
		return "", err
	}
	if phase != 0 {
		return "", problemf(ErrInternal, "提交差异输出不完整")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CommitFile reads an immutable file snapshot at a verified full hex commit OID
// within a session-independent repository or worktree context.
func (s *Service) CommitFile(ctx context.Context, t Target, cid, commitOID, nid string) (File, error) {
	return withTargetContext(ctx, s, t, cid, func(key scopeKey, b *binding) (File, error) {
		p, err := nodePath(nid)
		if err != nil {
			return File{}, err
		}
		if p == "" {
			return File{}, problemf(ErrInvalid, "文件标识不能为空")
		}
		if err := b.verifyCommitOID(ctx, commitOID); err != nil {
			return File{}, err
		}
		raw, err := git(ctx, b.gitCwd(), "ls-tree", "-z", commitOID, "--", p)
		if err != nil {
			return File{}, err
		}
		var mode, blobOID string
		for _, record := range strings.Split(string(raw), "\x00") {
			header, recPath, ok := strings.Cut(record, "\t")
			if !ok || recPath != p {
				continue
			}
			fields := strings.Fields(header)
			if len(fields) != 3 {
				return File{}, problemf(ErrInternal, "无效历史树记录")
			}
			if fields[1] == "tree" {
				return File{}, problemf(ErrInvalid, "目标是目录，不能作为文件读取")
			}
			mode, blobOID = fields[0], fields[2]
			break
		}
		if blobOID == "" {
			return File{}, problemf(ErrNotFound, "该提交中不存在此文件")
		}
		d, err := b.blob(ctx, p, commitSourcePrefix+commitOID, blobOID, mode)
		if err != nil {
			return File{}, err
		}
		f := File{Document: d, Checked: time.Now().UTC()}
		f.Snapshot = s.put(&snapshot{owner: ownerScope(key), context: cid, file: &f, size: documentSize(d) + 512})
		return f, nil
	})
}

// CommitChanges compares two verified full hex commit OIDs (fromOID as old side,
// toOID as new side) within a repository or worktree context without mutating
// HEAD, branches or index.
func (s *Service) CommitChanges(ctx context.Context, t Target, cid, fromOID, toOID string, queries ...ChangeQuery) (Changes, error) {
	return withTargetContext(ctx, s, t, cid, func(_ scopeKey, b *binding) (Changes, error) {
		r := Changes{Entries: []Change{}, Checked: time.Now().UTC()}
		diffRef, canonicalFrom, _, err := b.resolveFromTreeRef(ctx, fromOID)
		if err != nil {
			return r, err
		}
		if err := b.verifyCommitOID(ctx, toOID); err != nil {
			return r, err
		}
		var q ChangeQuery
		if len(queries) > 0 {
			q = queries[0]
		}
		var filterPath string
		if q.NodeID != "" {
			filterPath, err = nodePath(q.NodeID)
			if err != nil {
				return r, err
			}
		}
		start, tag := 0, ""
		if q.Cursor != "" {
			var offset string
			var ok bool
			tag, offset, ok = strings.Cut(q.Cursor, ":")
			start, err = strconv.Atoi(offset)
			if !ok || tag == "" || err != nil || start < 0 {
				return r, problemf(ErrInvalid, "无效修改游标")
			}
		}
		search := strings.ToLower(q.Q)
		ver, err := b.scanCommitChanges(ctx, diffRef, canonicalFrom, toOID, func(c Change) error {
			if q.NodeID != "" && c.NewPath != filterPath && c.OldPath != filterPath {
				return nil
			}
			if search != "" && !strings.Contains(strings.ToLower(displayPath(c.NewPath)), search) && !strings.Contains(strings.ToLower(displayPath(c.OldPath)), search) {
				return nil
			}
			if r.Total >= start && len(r.Entries) < ChangePageSize {
				r.Entries = append(r.Entries, c)
			}
			r.Total++
			return nil
		})
		if err != nil {
			return r, err
		}
		r.Version = ver
		expected := version([]string{cid, canonicalFrom, toOID, r.Version, q.Q, q.NodeID})
		if q.Cursor != "" && tag != expected {
			return r, problemf(ErrConflict, "修改游标不属于当前比较上下文")
		}
		if start > r.Total {
			return r, problemf(ErrInvalid, "无效修改页")
		}
		end := start + len(r.Entries)
		if end < r.Total {
			r.Next = fmt.Sprintf("%s:%d", expected, end)
		}
		return r, nil
	})
}

// CommitDiff captures a frozen two-sided diff snapshot for one changed file
// between verified full hex commits fromOID (old) and toOID (new).
func (s *Service) CommitDiff(ctx context.Context, t Target, cid, fromOID, toOID, changeID string) (Diff, error) {
	return withTargetContext(ctx, s, t, cid, func(key scopeKey, b *binding) (Diff, error) {
		diffRef, canonicalFrom, isEmpty, err := b.resolveFromTreeRef(ctx, fromOID)
		if err != nil {
			return Diff{}, err
		}
		if err := b.verifyCommitOID(ctx, toOID); err != nil {
			return Diff{}, err
		}
		var found *Change
		_, err = b.scanCommitChanges(ctx, diffRef, canonicalFrom, toOID, func(entry Change) error {
			if entry.ID == changeID {
				copyEntry := entry
				found = &copyEntry
			}
			return nil
		})
		if err != nil {
			return Diff{}, err
		}
		if found == nil {
			return Diff{}, problemf(ErrNotFound, "指定的历史变更项不存在")
		}
		if _, err := nodePath(nodeID(found.OldPath)); err != nil {
			return Diff{}, err
		}
		if _, err := nodePath(nodeID(found.NewPath)); err != nil {
			return Diff{}, err
		}
		oldSource := "empty"
		if !isEmpty {
			oldSource = commitSourcePrefix + canonicalFrom
		}
		oldDoc, err := b.blob(ctx, found.OldPath, oldSource, found.OldOID, found.OldMode)
		if err != nil {
			return Diff{}, err
		}
		newDoc, err := b.blob(ctx, found.NewPath, commitSourcePrefix+toOID, found.NewOID, found.NewMode)
		if err != nil {
			return Diff{}, err
		}
		diff := Diff{
			Old:     oldDoc,
			New:     newDoc,
			Kind:    "text",
			Checked: time.Now().UTC(),
			Change:  *found,
		}
		if kind, reason := classify(oldDoc, newDoc); kind != "text" {
			diff.Kind = kind
			diff.Reason = reason
		} else {
			transformed, attrErr := b.hasCommitTransformAttr(ctx, toOID, found.NewPath)
			if attrErr != nil {
				return Diff{}, attrErr
			}
			if !transformed && !isEmpty && found.OldPath != found.NewPath {
				transformed, attrErr = b.hasCommitTransformAttr(ctx, canonicalFrom, found.OldPath)
				if attrErr != nil {
					return Diff{}, attrErr
				}
			}
			if transformed {
				diff.Kind = "metadata"
				diff.Reason = "需要内容转换，不能保证原始行映射"
			}
		}
		snap := &snapshot{owner: ownerScope(key), context: cid, diff: &diff, size: documentSize(oldDoc) + documentSize(newDoc)}
		diff.Snapshot = s.put(snap)
		return diff, nil
	})
}
