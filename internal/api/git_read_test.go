package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

type gitAPITestEnv struct {
	t       *testing.T
	server  *httptest.Server
	manager *session.Manager
	ws      *workspace.Service
	root    string
	repo    contract.Repository
	mainWT  contract.Worktree
}

func newGitAPITestEnv(t *testing.T) *gitAPITestEnv {
	t.Helper()
	root := gitRepository(t)
	c := config.Config{Listen: "127.0.0.1:7681", DefaultCwd: root, Token: "secret"}
	m := session.NewManager(testSessionOptions(c))
	ws := testWorkspace(t, m, root)
	server := httptest.NewServer(New(testDeps(t, testServices{Config: c, Manager: m, Workspace: ws})))
	t.Cleanup(func() {
		server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	env := &gitAPITestEnv{t: t, server: server, manager: m, ws: ws, root: root}
	raw := env.doJSON("POST", "/api/repositories", map[string]string{"path": root}, 201)
	if err := json.Unmarshal(raw, &env.repo); err != nil {
		t.Fatal(err)
	}
	if len(env.repo.Worktrees) == 0 {
		t.Fatal("expected main worktree")
	}
	env.mainWT = env.repo.Worktrees[0]
	return env
}

func (e *gitAPITestEnv) gitOut(cwd string, args ...string) string {
	e.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	out, err := testCommandOutput(cmd)
	if err != nil {
		e.t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(out)
}

func (e *gitAPITestEnv) doRaw(method, path string, body any, headers map[string]string, host string) *http.Response {
	e.t.Helper()
	var reader io.Reader
	switch v := body.(type) {
	case nil:
	case string:
		reader = strings.NewReader(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			e.t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		e.t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	return res
}

func (e *gitAPITestEnv) doJSON(method, path string, body any, want int) []byte {
	e.t.Helper()
	res := e.doRaw(method, path, body, map[string]string{"Authorization": "Bearer secret"}, "")
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	if res.StatusCode != want {
		e.t.Fatalf("%s %s: status %d want %d: %s", method, path, res.StatusCode, want, data)
	}
	if strings.Contains(path, "/git/") && res.Header.Get("Cache-Control") != "no-store" {
		e.t.Fatalf("%s %s: Cache-Control = %q, want no-store", method, path, res.Header.Get("Cache-Control"))
	}
	return data
}

func encodeNodeID(path string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(path))
}

func TestGitReadAPIAuthOriginAndZeroSessions(t *testing.T) {
	env := newGitAPITestEnv(t)
	if got := len(env.manager.List()); got != 0 {
		t.Fatalf("expected zero active sessions, got %d", got)
	}

	// Build two commits + staged + unstaged + untracked + ignored + PNG image.
	var pngBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, ".gitignore"), []byte("ignored.log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "app.txt"), []byte("v1\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "logo.png"), pngBuf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", ".")
	env.gitOut(env.root, "commit", "-m", "feat: add app and logo\n\ndetailed body line")
	firstOID := env.gitOut(env.root, "rev-parse", "HEAD")

	if err := os.WriteFile(filepath.Join(env.root, "app.txt"), []byte("v2-committed\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "app.txt")
	env.gitOut(env.root, "commit", "-m", "feat: update app to v2")
	secondOID := env.gitOut(env.root, "rev-parse", "HEAD")

	// Staged version differs from HEAD, worktree version differs from index.
	if err := os.WriteFile(filepath.Join(env.root, "app.txt"), []byte("v3-staged\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "app.txt")
	if err := os.WriteFile(filepath.Join(env.root, "app.txt"), []byte("v4-worktree\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "untracked.txt"), []byte("new file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "ignored.log"), []byte("secret log\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	base := "/api/repositories/" + env.repo.ID + "/git"

	// 401 & 403 boundaries on read endpoints.
	for _, tc := range []struct {
		name    string
		path    string
		headers map[string]string
		host    string
		want    int
	}{
		{"missing token", base + "/context", nil, "", 401},
		{"wrong token", base + "/context", map[string]string{"Authorization": "Bearer wrong"}, "", 401},
		{"query token rejected", base + "/context?token=secret", nil, "", 401},
		{"cookie token rejected", base + "/context", map[string]string{"Cookie": "token=secret"}, "", 401},
		{"evil origin", base + "/context", map[string]string{"Authorization": "Bearer secret", "Origin": "https://evil.example"}, "", 403},
		{"null origin", base + "/history", map[string]string{"Authorization": "Bearer secret", "Origin": "null"}, "", 403},
		{"bad host", base + "/commits/" + secondOID, map[string]string{"Authorization": "Bearer secret"}, "evil.example:7681", 403},
	} {
		res := env.doRaw("GET", tc.path, nil, tc.headers, tc.host)
		res.Body.Close()
		if res.StatusCode != tc.want {
			t.Fatalf("%s: got %d want %d", tc.name, res.StatusCode, tc.want)
		}
	}

	// 1. Repository-only context (no worktreeId): compare & commit:{oid} work; worktree browse/changes return 409 readonly.
	var repoCtx contract.GitContext
	if err := json.Unmarshal(env.doJSON("GET", base+"/context", nil, 200), &repoCtx); err != nil {
		t.Fatal(err)
	}
	if repoCtx.Kind != "repository" || repoCtx.Browse || repoCtx.Diff || repoCtx.Write {
		t.Fatalf("unexpected repo context: %+v", repoCtx)
	}
	for _, blocked := range []string{
		base + "/tree?contextId=" + repoCtx.ID,
		base + "/files?contextId=" + repoCtx.ID + "&q=app",
		base + "/changes?contextId=" + repoCtx.ID,
		base + "/file?contextId=" + repoCtx.ID + "&nodeId=" + encodeNodeID("app.txt") + "&source=worktree",
	} {
		var conflict contract.GitConflict
		if err := json.Unmarshal(env.doJSON("GET", blocked, nil, 409), &conflict); err != nil {
			t.Fatal(err)
		}
		if conflict.Reason != "readonly" || conflict.Error == "" {
			t.Fatalf("%s: expected readonly conflict, got %+v", blocked, conflict)
		}
	}

	var commitFile contract.GitFile
	if err := json.Unmarshal(env.doJSON("GET", base+"/file?contextId="+repoCtx.ID+"&nodeId="+encodeNodeID("app.txt")+"&source=commit:"+firstOID, nil, 200), &commitFile); err != nil {
		t.Fatal(err)
	}
	if commitFile.Document.Text != "v1\nline2\n" || commitFile.Document.Source != "commit:"+firstOID {
		t.Fatalf("commit file from repo context: %+v", commitFile)
	}

	var compared contract.GitCompare
	compareURL := base + "/compare?" + url.Values{
		"contextId": {repoCtx.ID},
		"baseOid":   {firstOID},
		"targetOid": {secondOID},
		"path":      {"app.txt"},
	}.Encode()
	if err := json.Unmarshal(env.doJSON("GET", compareURL, nil, 200), &compared); err != nil {
		t.Fatal(err)
	}
	if compared.Changes.Total != 1 || compared.Diff == nil || compared.Diff.Old.Text != "v1\nline2\n" || compared.Diff.New.Text != "v2-committed\nline2\n" {
		t.Fatalf("compare response: %+v", compared)
	}
	// Swapped direction reverses before/after.
	var swapped contract.GitCompare
	swapURL := base + "/compare?" + url.Values{
		"contextId": {repoCtx.ID},
		"baseOid":   {secondOID},
		"targetOid": {firstOID},
		"path":      {"app.txt"},
	}.Encode()
	if err := json.Unmarshal(env.doJSON("GET", swapURL, nil, 200), &swapped); err != nil {
		t.Fatal(err)
	}
	if swapped.Diff == nil || swapped.Diff.Old.Text != "v2-committed\nline2\n" || swapped.Diff.New.Text != "v1\nline2\n" {
		t.Fatalf("swapped compare: %+v", swapped)
	}
	// Snapshot file with side=before and side=after.
	var beforeSnap, afterSnap contract.GitFile
	if err := json.Unmarshal(env.doJSON("GET", base+"/snapshots/"+compared.Diff.Snapshot+"/file?side=before", nil, 200), &beforeSnap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(env.doJSON("GET", base+"/snapshots/"+compared.Diff.Snapshot+"/file?side=after", nil, 200), &afterSnap); err != nil {
		t.Fatal(err)
	}
	if beforeSnap.Document.Text != "v1\nline2\n" || afterSnap.Document.Text != "v2-committed\nline2\n" {
		t.Fatalf("snapshot sides: before=%q after=%q", beforeSnap.Document.Text, afterSnap.Document.Text)
	}

	// 2. Worktree context: tree, search, 4 file sources, changes, diff, raw GET/HEAD.
	var wtCtx contract.GitContext
	if err := json.Unmarshal(env.doJSON("GET", base+"/context?worktreeId="+env.mainWT.ID, nil, 200), &wtCtx); err != nil {
		t.Fatal(err)
	}
	if wtCtx.Kind != "worktree" || !wtCtx.Browse || !wtCtx.Diff || !wtCtx.Write {
		t.Fatalf("unexpected worktree context: %+v", wtCtx)
	}
	var tree contract.CodeListing
	if err := json.Unmarshal(env.doJSON("GET", base+"/tree?contextId="+wtCtx.ID, nil, 200), &tree); err != nil {
		t.Fatal(err)
	}
	for _, child := range tree.Children {
		if child.Path == "ignored.log" {
			t.Fatal("ignored file returned when includeIgnored=false")
		}
	}
	var treeWithIgnored contract.CodeListing
	if err := json.Unmarshal(env.doJSON("GET", base+"/tree?contextId="+wtCtx.ID+"&includeIgnored=true", nil, 200), &treeWithIgnored); err != nil {
		t.Fatal(err)
	}
	foundIgnored := false
	for _, child := range treeWithIgnored.Children {
		if child.Path == "ignored.log" {
			foundIgnored = true
		}
	}
	if !foundIgnored {
		t.Fatal("expected ignored.log when includeIgnored=true")
	}
	env.doJSON("GET", base+"/tree?contextId="+wtCtx.ID+"&includeIgnored=invalid", nil, 400)

	var searched contract.CodeListing
	if err := json.Unmarshal(env.doJSON("GET", base+"/files?contextId="+wtCtx.ID+"&q=logo", nil, 200), &searched); err != nil {
		t.Fatal(err)
	}
	if len(searched.Children) != 1 || searched.Children[0].Path != "logo.png" {
		t.Fatalf("search logo: %+v", searched.Children)
	}

	for source, wantText := range map[string]string{
		"worktree":           "v4-worktree\nline2\n",
		"index":              "v3-staged\nline2\n",
		"head":               "v2-committed\nline2\n",
		"commit:" + firstOID: "v1\nline2\n",
	} {
		var got contract.GitFile
		u := base + "/file?contextId=" + wtCtx.ID + "&nodeId=" + encodeNodeID("app.txt") + "&source=" + url.QueryEscape(source)
		if err := json.Unmarshal(env.doJSON("GET", u, nil, 200), &got); err != nil {
			t.Fatal(err)
		}
		if got.Document.Text != wantText {
			t.Fatalf("source %s: got %q want %q", source, got.Document.Text, wantText)
		}
	}

	// Changes & working diff.
	var changes contract.GitChanges
	if err := json.Unmarshal(env.doJSON("GET", base+"/changes?contextId="+wtCtx.ID+"&group=staged", nil, 200), &changes); err != nil {
		t.Fatal(err)
	}
	if len(changes.Entries) != 1 || changes.Entries[0].NewPath != "app.txt" {
		t.Fatalf("staged changes: %+v", changes)
	}
	var wtDiff contract.GitDiff
	if err := json.Unmarshal(env.doJSON("GET", base+"/diff?contextId="+wtCtx.ID+"&changeId="+changes.Entries[0].ID, nil, 200), &wtDiff); err != nil {
		t.Fatal(err)
	}
	if wtDiff.Old.Text != "v2-committed\nline2\n" || wtDiff.New.Text != "v3-staged\nline2\n" {
		t.Fatalf("staged diff: old=%q new=%q", wtDiff.Old.Text, wtDiff.New.Text)
	}

	// Binary snapshot raw GET & HEAD.
	var logoFile contract.GitFile
	if err := json.Unmarshal(env.doJSON("GET", base+"/file?contextId="+wtCtx.ID+"&nodeId="+encodeNodeID("logo.png"), nil, 200), &logoFile); err != nil {
		t.Fatal(err)
	}
	rawPath := base + "/snapshots/" + logoFile.Snapshot + "/raw?side=after"
	rawRes := env.doRaw("GET", rawPath+"&download=1", nil, map[string]string{"Authorization": "Bearer secret"}, "")
	rawBytes, err := io.ReadAll(rawRes.Body)
	rawRes.Body.Close()
	if err != nil || rawRes.StatusCode != 200 || !bytes.Equal(rawBytes, pngBuf.Bytes()) {
		t.Fatalf("raw GET: status=%d len=%d err=%v", rawRes.StatusCode, len(rawBytes), err)
	}
	if rawRes.Header.Get("Cache-Control") != "no-store" || rawRes.Header.Get("Content-Type") != "image/png" || !strings.HasPrefix(rawRes.Header.Get("Content-Disposition"), "attachment;") {
		t.Fatalf("raw headers: %v", rawRes.Header)
	}
	headRes := env.doRaw("HEAD", rawPath, nil, map[string]string{"Authorization": "Bearer secret"}, "")
	headBody, _ := io.ReadAll(headRes.Body)
	headRes.Body.Close()
	if headRes.StatusCode != 200 || len(headBody) != 0 || headRes.Header.Get("Content-Length") != strconv.Itoa(pngBuf.Len()) {
		t.Fatalf("raw HEAD: status=%d body=%d headers=%v", headRes.StatusCode, len(headBody), headRes.Header)
	}
	env.doJSON("GET", base+"/snapshots/"+logoFile.Snapshot+"/raw?side=middle", nil, 400)
}

func TestGitReadAPIHistoryCommitDetailAndIsolation(t *testing.T) {
	env := newGitAPITestEnv(t)
	rootOID := env.gitOut(env.root, "rev-parse", "HEAD")

	// Create a feature branch and merge it with --no-ff so we have a real 2-parent merge commit.
	if err := os.WriteFile(filepath.Join(env.root, "base.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "base.txt")
	env.gitOut(env.root, "commit", "-m", "feat: base file")
	mainParentOID := env.gitOut(env.root, "rev-parse", "HEAD")

	env.gitOut(env.root, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(env.root, "feature.txt"), []byte("feature\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "feature.txt")
	env.gitOut(env.root, "commit", "-m", "feat: feature branch file")
	featureOID := env.gitOut(env.root, "rev-parse", "HEAD")

	env.gitOut(env.root, "checkout", "main")
	if err := os.WriteFile(filepath.Join(env.root, "main-only.txt"), []byte("main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "main-only.txt")
	env.gitOut(env.root, "commit", "-m", "feat: main branch file")
	firstParentOID := env.gitOut(env.root, "rev-parse", "HEAD")

	env.gitOut(env.root, "merge", "--no-ff", "feature", "-m", "merge: integrate feature\n\nmerge body")
	mergeOID := env.gitOut(env.root, "rev-parse", "HEAD")

	// Set up bare remote with one incoming commit and one local unpushed commit.
	bareRemote := filepath.Join(t.TempDir(), "remote.git")
	env.gitOut(env.root, "clone", "--bare", env.root, bareRemote)
	env.gitOut(env.root, "remote", "add", "origin", bareRemote)
	env.gitOut(env.root, "push", "-u", "origin", "main")

	// Advance origin/main with an incoming commit, then fetch.
	incomingTree := env.gitOut(bareRemote, "rev-parse", "main^{tree}")
	incomingCommit := env.gitOut(bareRemote, "commit-tree", "-p", "main", "-m", "chore: upstream commit", incomingTree)
	env.gitOut(bareRemote, "update-ref", "refs/heads/main", incomingCommit)
	env.gitOut(env.root, "fetch", "origin")

	// Advance local main with a local unpushed commit.
	if err := os.WriteFile(filepath.Join(env.root, "unpushed.txt"), []byte("local ahead\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "unpushed.txt")
	env.gitOut(env.root, "commit", "-m", "feat: local unpushed")
	localTipOID := env.gitOut(env.root, "rev-parse", "HEAD")
	_ = mainParentOID

	base := "/api/repositories/" + env.repo.ID + "/git"

	// Paginated history + graph edges.
	var page1, page2 contract.GitHistoryPage
	if err := json.Unmarshal(env.doJSON("GET", base+"/history?scope=all&limit=2", nil, 200), &page1); err != nil {
		t.Fatal(err)
	}
	if page1.Status != "ready" || len(page1.Rows) != 2 || page1.NextCursor == "" {
		t.Fatalf("history page1: %+v", page1)
	}
	if err := json.Unmarshal(env.doJSON("GET", base+"/history?scope=all&limit=2&cursor="+url.QueryEscape(page1.NextCursor), nil, 200), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Rows) == 0 || page2.Rows[0].OID != mergeOID || len(page2.Rows[0].Parents) != 2 {
		t.Fatalf("expected merge row with 2 parents at start of page2, got %+v", page2)
	}

	// to-push & incoming scopes.
	var toPush, incoming, noWT contract.GitHistoryPage
	if err := json.Unmarshal(env.doJSON("GET", base+"/history?scope=to-push&worktreeId="+env.mainWT.ID, nil, 200), &toPush); err != nil {
		t.Fatal(err)
	}
	if toPush.Status != "ready" || len(toPush.Rows) != 1 || toPush.Rows[0].OID != localTipOID {
		t.Fatalf("to-push: %+v", toPush)
	}
	if err := json.Unmarshal(env.doJSON("GET", base+"/history?scope=incoming&worktreeId="+env.mainWT.ID, nil, 200), &incoming); err != nil {
		t.Fatal(err)
	}
	if incoming.Status != "ready" || len(incoming.Rows) != 1 || incoming.Rows[0].OID != incomingCommit {
		t.Fatalf("incoming: %+v", incoming)
	}
	if err := json.Unmarshal(env.doJSON("GET", base+"/history?scope=to-push", nil, 200), &noWT); err != nil {
		t.Fatal(err)
	}
	if noWT.Status != "no_worktree" {
		t.Fatalf("expected no_worktree without worktreeId, got %+v", noWT)
	}
	env.doJSON("GET", base+"/history?limit=0", nil, 400)
	env.doJSON("GET", base+"/history?scope=invalid", nil, 400)

	// Commit details: root commit, merge commit default parent vs second parent, invalid parent, injection.
	var rootDetail contract.GitCommitDetail
	if err := json.Unmarshal(env.doJSON("GET", base+"/commits/"+rootOID, nil, 200), &rootDetail); err != nil {
		t.Fatal(err)
	}
	if rootDetail.ComparedParentOID != "" || len(rootDetail.Parents) != 0 {
		t.Fatalf("root commit detail: %+v", rootDetail)
	}

	var mergeP1, mergeP2 contract.GitCommitDetail
	if err := json.Unmarshal(env.doJSON("GET", base+"/commits/"+mergeOID, nil, 200), &mergeP1); err != nil {
		t.Fatal(err)
	}
	if mergeP1.ComparedParentOID != firstParentOID || !strings.Contains(mergeP1.Message, "merge body") || len(mergeP1.Files) != 1 || mergeP1.Files[0].NewPath != "feature.txt" {
		t.Fatalf("merge parent 1 detail: %+v", mergeP1)
	}
	if err := json.Unmarshal(env.doJSON("GET", base+"/commits/"+mergeOID+"?parentOid="+featureOID, nil, 200), &mergeP2); err != nil {
		t.Fatal(err)
	}
	if mergeP2.ComparedParentOID != featureOID || len(mergeP2.Files) != 1 || mergeP2.Files[0].NewPath != "main-only.txt" {
		t.Fatalf("merge parent 2 detail: %+v", mergeP2)
	}
	env.doJSON("GET", base+"/commits/"+mergeOID+"?parentOid="+rootOID, nil, 400)
	env.doJSON("GET", base+"/commits/HEAD~1", nil, 400)
	env.doJSON("GET", base+"/commits/--output=pwn", nil, 400)
	env.doJSON("GET", base+"/commits/deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil, 404)

	// Bare repository registration + context + cross-repo & cross-worktree isolation.
	var bareRepo contract.Repository
	if err := json.Unmarshal(env.doJSON("POST", "/api/repositories", map[string]string{"path": bareRemote}, 201), &bareRepo); err != nil {
		t.Fatal(err)
	}
	var bareCtx contract.GitContext
	if err := json.Unmarshal(env.doJSON("GET", "/api/repositories/"+bareRepo.ID+"/git/context", nil, 200), &bareCtx); err != nil {
		t.Fatal(err)
	}
	if bareCtx.Kind != "bare" || bareCtx.Browse {
		t.Fatalf("bare context: %+v", bareCtx)
	}
	var bareReadOnly contract.GitConflict
	if err := json.Unmarshal(env.doJSON("GET", "/api/repositories/"+bareRepo.ID+"/git/tree?contextId="+bareCtx.ID, nil, 409), &bareReadOnly); err != nil || bareReadOnly.Reason != "readonly" {
		t.Fatalf("bare tree conflict: %+v %v", bareReadOnly, err)
	}

	// Cross-repository contextId -> 404.
	var mainCtx contract.GitContext
	if err := json.Unmarshal(env.doJSON("GET", base+"/context?worktreeId="+env.mainWT.ID, nil, 200), &mainCtx); err != nil {
		t.Fatal(err)
	}
	env.doJSON("GET", "/api/repositories/"+bareRepo.ID+"/git/file?contextId="+mainCtx.ID+"&nodeId="+encodeNodeID("base.txt")+"&source=commit:"+mergeOID, nil, 404)

	// Cross-worktree contextId & snapshotId -> 409 context_mismatch.
	var linkedWT contract.Worktree
	linkedRaw := env.doJSON("POST", "/api/repositories/"+env.repo.ID+"/worktrees", contract.CreateWorktreeRequest{
		Path: filepath.Join(t.TempDir(), "linked-wt"), Mode: "new", Branch: "linked-wt", Base: "HEAD",
	}, 201)
	if err := json.Unmarshal(linkedRaw, &linkedWT); err != nil {
		t.Fatal(err)
	}
	var ctxMismatch contract.GitConflict
	if err := json.Unmarshal(env.doJSON("GET", base+"/tree?worktreeId="+linkedWT.ID+"&contextId="+mainCtx.ID, nil, 409), &ctxMismatch); err != nil || ctxMismatch.Reason != "context_mismatch" {
		t.Fatalf("expected context_mismatch, got %+v (%v)", ctxMismatch, err)
	}
	var snapFile contract.GitFile
	if err := json.Unmarshal(env.doJSON("GET", base+"/file?contextId="+mainCtx.ID+"&nodeId="+encodeNodeID("base.txt"), nil, 200), &snapFile); err != nil {
		t.Fatal(err)
	}
	var snapMismatch contract.GitConflict
	if err := json.Unmarshal(env.doJSON("GET", base+"/snapshots/"+snapFile.Snapshot+"/file?worktreeId="+linkedWT.ID+"&side=after", nil, 409), &snapMismatch); err != nil || snapMismatch.Reason != "context_mismatch" {
		t.Fatalf("expected snapshot context_mismatch, got %+v (%v)", snapMismatch, err)
	}
	// Cross-repository snapshotId -> 404.
	env.doJSON("GET", "/api/repositories/"+bareRepo.ID+"/git/snapshots/"+snapFile.Snapshot+"/file?side=after", nil, 404)
	// Expired/unknown contextId & snapshotId -> 409 stale.
	var staleCtx contract.GitConflict
	if err := json.Unmarshal(env.doJSON("GET", base+"/tree?contextId=expired00", nil, 409), &staleCtx); err != nil || staleCtx.Reason != "stale" {
		t.Fatalf("expected stale context, got %+v (%v)", staleCtx, err)
	}
	var staleSnap contract.GitConflict
	if err := json.Unmarshal(env.doJSON("GET", base+"/snapshots/expired00/file?side=after", nil, 409), &staleSnap); err != nil || staleSnap.Reason != "stale" {
		t.Fatalf("expected stale snapshot, got %+v (%v)", staleSnap, err)
	}
	// Path traversal -> 400.
	for _, badPath := range []string{"../outside", "/etc/passwd", ".git/config"} {
		env.doJSON("GET", base+"/file?contextId="+mainCtx.ID+"&nodeId="+encodeNodeID(badPath), nil, 400)
	}
}
