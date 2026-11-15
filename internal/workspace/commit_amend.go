package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CommitPreviewOptions controls explicit advanced commit options (all default false/empty).
type CommitPreviewOptions struct {
	Amend          bool   `json:"amend,omitempty"`
	AuthorOverride string `json:"authorOverride,omitempty"`
	Signoff        bool   `json:"signoff,omitempty"`
}

type commitFullMeta struct {
	TreeOID        string
	Parents        []string
	Subject        string
	Message        string
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
}

// parseAuthorOverride validates an optional `Name <email>` override string.
// Empty/whitespace input returns ("", "", "", nil).
func parseAuthorOverride(raw string) (canonical, name, email string, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", "", nil
	}
	if strings.ContainsAny(trimmed, "\x00\r\n") {
		return "", "", "", problemf(ErrInvalid, "作者覆盖包含非法控制字符")
	}
	lt := strings.LastIndexByte(trimmed, '<')
	if lt <= 0 || !strings.HasSuffix(trimmed, ">") {
		return "", "", "", problemf(ErrInvalid, "作者覆盖格式无效，请使用 Name <email> 格式（例如 Alice <alice@example.com>）")
	}
	name = strings.TrimSpace(trimmed[:lt])
	email = strings.TrimSpace(trimmed[lt+1 : len(trimmed)-1])
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "<>") {
		return "", "", "", problemf(ErrInvalid, "作者姓名无效，请使用 Name <email> 格式")
	}
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" || strings.HasPrefix(email, "-") || strings.ContainsAny(local, " \t<>@") || strings.ContainsAny(domain, " \t<>@") {
		return "", "", "", problemf(ErrInvalid, "作者邮箱格式无效，请使用 Name <email> 格式")
	}
	return fmt.Sprintf("%s <%s>", name, email), name, email, nil
}

func computeSignoffTrailer(ident CommitIdentity) string {
	name := strings.TrimSpace(ident.CommitterName)
	email := strings.TrimSpace(ident.CommitterEmail)
	if name == "" || email == "" {
		return ""
	}
	return fmt.Sprintf("Signed-off-by: %s <%s>", name, email)
}

func (s *Service) readCommitFullMeta(ctx context.Context, cwd, oid string) (commitFullMeta, error) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput,
		"log", "-n", "1", "--no-show-signature",
		"--format=%T%x00%P%x00%s%x00%B%x00%an%x00%ae%x00%cn%x00%ce%x00",
		oid, "--",
	)
	if err != nil {
		return commitFullMeta{}, err
	}
	fields := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	if len(fields) < 8 {
		return commitFullMeta{}, problemf(ErrInternal, "提交元数据格式无效")
	}
	var parents []string
	if rawParents := strings.TrimSpace(string(fields[1])); rawParents != "" {
		parents = strings.Fields(rawParents)
	} else {
		parents = []string{}
	}
	return commitFullMeta{
		TreeOID:        strings.TrimSpace(string(fields[0])),
		Parents:        parents,
		Subject:        string(fields[2]),
		Message:        strings.TrimRight(string(fields[3]), "\r\n"),
		AuthorName:     string(fields[4]),
		AuthorEmail:    string(fields[5]),
		CommitterName:  string(fields[6]),
		CommitterEmail: string(fields[7]),
	}, nil
}

func (s *Service) readCommitTreeAndParents(ctx context.Context, cwd, oid string) (treeOID string, parents []string, subject string, err error) {
	meta, err := s.readCommitFullMeta(ctx, cwd, oid)
	if err != nil {
		return "", nil, "", err
	}
	return meta.TreeOID, meta.Parents, meta.Subject, nil
}

func (s *Service) readTreeOIDForCommit(ctx context.Context, cwd, commitOrTree string) (string, error) {
	data, err := ReadGit(ctx, cwd, 256, "rev-parse", "--verify", "--end-of-options", commitOrTree+"^{tree}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Service) readCommitIdentity(ctx context.Context, cwd string) (CommitIdentity, error) {
	userName, _, err := s.configValue(ctx, cwd, "user.name")
	if err != nil {
		return CommitIdentity{}, err
	}
	userEmail, _, err := s.configValue(ctx, cwd, "user.email")
	if err != nil {
		return CommitIdentity{}, err
	}
	authorNameConf, _, _ := s.configValue(ctx, cwd, "author.name")
	authorEmailConf, _, _ := s.configValue(ctx, cwd, "author.email")
	committerNameConf, _, _ := s.configValue(ctx, cwd, "committer.name")
	committerEmailConf, _, _ := s.configValue(ctx, cwd, "committer.email")
	gpgSign, _, _ := s.configValue(ctx, cwd, "commit.gpgsign")
	gpgFormat, _, _ := s.configValue(ctx, cwd, "gpg.format")

	pick := func(values ...string) string {
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				return v
			}
		}
		return ""
	}
	signEnabled := strings.EqualFold(gpgSign, "true") || gpgSign == "1" || strings.EqualFold(gpgSign, "yes")
	authorName := pick(os.Getenv("GIT_AUTHOR_NAME"), authorNameConf, userName)
	authorEmail := pick(os.Getenv("GIT_AUTHOR_EMAIL"), authorEmailConf, userEmail, os.Getenv("EMAIL"))
	return CommitIdentity{
		AuthorName:     authorName,
		AuthorEmail:    authorEmail,
		EnvAuthorName:  authorName,
		EnvAuthorEmail: authorEmail,
		CommitterName:  pick(os.Getenv("GIT_COMMITTER_NAME"), committerNameConf, userName),
		CommitterEmail: pick(os.Getenv("GIT_COMMITTER_EMAIL"), committerEmailConf, userEmail, os.Getenv("EMAIL")),
		SignEnabled:    signEnabled,
		SignFormat:     gpgFormat,
	}, nil
}

func (s *Service) detectCommitHooks(ctx context.Context, cwd string) []string {
	active := make([]string, 0, len(commitHookNames))
	for _, name := range commitHookNames {
		data, err := ReadGit(ctx, cwd, 1024, "rev-parse", "--git-path", "hooks/"+name)
		if err != nil {
			continue
		}
		p := outputLine(data)
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			active = append(active, name)
		}
	}
	return active
}

func (s *Service) scanCommitReflogWithBranch(ctx context.Context, wtPath string, wtReachable bool, anchorPath, branch, reflogTag string) ([]string, error) {
	seen := make(map[string]bool)
	matches := make([]string, 0, 2)
	addMatches := func(list []string) {
		for _, oid := range list {
			if !seen[oid] {
				seen[oid] = true
				matches = append(matches, oid)
			}
		}
	}
	var firstErr error
	if wtReachable {
		headMatches, err := s.scanReflogRef(ctx, wtPath, "HEAD", reflogTag)
		if err != nil {
			firstErr = err
		} else {
			addMatches(headMatches)
		}
	}
	if branch != "" {
		cwd := anchorPath
		if wtReachable {
			cwd = wtPath
		}
		branchMatches, err := s.scanReflogRef(ctx, cwd, "refs/heads/"+branch, reflogTag)
		if err == nil {
			addMatches(branchMatches)
			if len(branchMatches) > 0 {
				firstErr = nil
			}
		} else if firstErr == nil {
			firstErr = err
		}
	}
	return matches, firstErr
}

func (s *Service) scanReflogRef(ctx context.Context, cwd, ref, reflogTag string) ([]string, error) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "log", "-g", "-n", "64", "--no-show-signature", "--format=%H%x00%gs%x00", ref, "--")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	fields := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	matches := make([]string, 0, 2)
	for i := 0; i+1 < len(fields); i += 2 {
		oid := strings.TrimLeft(string(fields[i]), "\r\n")
		gs := string(fields[i+1])
		if !isObjectID(oid) {
			continue
		}
		if gs == reflogTag || strings.HasPrefix(gs, reflogTag+":") || strings.HasPrefix(gs, reflogTag+" ") {
			matches = append(matches, oid)
		}
	}
	return matches, nil
}

func (s *Service) isAncestorReadOnly(ctx context.Context, cwd, ancestor, descendant string) bool {
	if s.gitPath == "" {
		return false
	}
	_, run := s.runGit(ctx, cwd, []string{"GIT_OPTIONAL_LOCKS=0"}, "merge-base", "--is-ancestor", ancestor, descendant)
	return run.err == nil
}
