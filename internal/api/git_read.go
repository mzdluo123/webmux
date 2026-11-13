package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
)

const gitReadTimeout = 10 * time.Second

// serveGit answers a read-only Git workbench JSON route within the 10 s bound
// and sets Cache-Control: no-store. archtest rule R8 verifies T at every call.
func serveGit[T any](mux *http.ServeMux, pattern string, fn func(context.Context, *http.Request) (T, error)) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), gitReadTimeout)
		defer cancel()
		value, err := fn(ctx, r)
		if err != nil {
			gitFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusOK, value)
	})
}

func parseOptionalBool(raw string) (bool, error) {
	switch raw {
	case "", "false", "0":
		return false, nil
	case "true", "1":
		return true, nil
	default:
		return false, &badRequest{err: errors.New("invalid boolean query parameter")}
	}
}

func parseOptionalPositiveInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, &badRequest{err: errors.New("invalid positive integer query parameter")}
	}
	return n, nil
}

func (s *Server) gitRoutes(mux *http.ServeMux) {
	s.gitReadRoutes(mux)
	s.gitWriteRoutes(mux)
	s.gitDivergenceRoutes(mux)
	s.gitStashRoutes(mux)
	s.gitV2Routes(mux)
	s.gitTaskRoutes(mux)
	s.gitCollabRoutes(mux)
}

func (s *Server) gitReadRoutes(mux *http.ServeMux) {
	const base = " /api/repositories/{id}/git"

	serveGit(mux, "GET"+base+"/context", func(ctx context.Context, r *http.Request) (contract.GitContext, error) {
		v, err := s.Code.GitContext(ctx, r.PathValue("id"), r.URL.Query().Get("worktreeId"))
		return wire.GitContext(v), err
	})

	serveGit(mux, "GET"+base+"/tree", func(ctx context.Context, r *http.Request) (contract.CodeListing, error) {
		q := r.URL.Query()
		include, err := parseOptionalBool(q.Get("includeIgnored"))
		if err != nil {
			return contract.CodeListing{}, err
		}
		v, err := s.Code.GitTree(ctx, r.PathValue("id"), q.Get("worktreeId"), q.Get("contextId"), q.Get("nodeId"), include, q.Get("cursor"))
		return wire.CodeListing(v), err
	})

	serveGit(mux, "GET"+base+"/files", func(ctx context.Context, r *http.Request) (contract.CodeListing, error) {
		q := r.URL.Query()
		include, err := parseOptionalBool(q.Get("includeIgnored"))
		if err != nil {
			return contract.CodeListing{}, err
		}
		v, err := s.Code.GitSearch(ctx, r.PathValue("id"), q.Get("worktreeId"), q.Get("contextId"), q.Get("q"), include)
		return wire.CodeListing(v), err
	})

	serveGit(mux, "GET"+base+"/file", func(ctx context.Context, r *http.Request) (contract.GitFile, error) {
		q := r.URL.Query()
		source := q.Get("source")
		if source == "" {
			source = "worktree"
		}
		v, err := s.Code.GitFile(ctx, r.PathValue("id"), q.Get("worktreeId"), q.Get("contextId"), q.Get("nodeId"), source)
		return wire.GitFile(v), err
	})

	serveGit(mux, "GET"+base+"/changes", func(ctx context.Context, r *http.Request) (contract.GitChanges, error) {
		q := r.URL.Query()
		v, err := s.Code.GitChanges(ctx, r.PathValue("id"), q.Get("worktreeId"), q.Get("contextId"), codeview.ChangeQuery{
			Cursor: q.Get("cursor"),
			Q:      q.Get("q"),
			NodeID: q.Get("nodeId"),
			Group:  q.Get("group"),
		})
		return wire.GitChanges(v), err
	})

	serveGit(mux, "GET"+base+"/diff", func(ctx context.Context, r *http.Request) (contract.GitDiff, error) {
		q := r.URL.Query()
		v, err := s.Code.GitDiff(ctx, r.PathValue("id"), q.Get("worktreeId"), q.Get("contextId"), q.Get("changeId"))
		return wire.GitDiff(v), err
	})

	serveGit(mux, "GET"+base+"/compare", func(ctx context.Context, r *http.Request) (contract.GitCompare, error) {
		q := r.URL.Query()
		v, err := s.Code.GitCompare(ctx, r.PathValue("id"), q.Get("worktreeId"), codeview.CompareQuery{
			ContextID: q.Get("contextId"),
			BaseOID:   q.Get("baseOid"),
			TargetOID: q.Get("targetOid"),
			Path:      q.Get("path"),
			ChangeID:  q.Get("changeId"),
			Q:         q.Get("q"),
			Cursor:    q.Get("cursor"),
		})
		return wire.GitCompare(v), err
	})

	serveGit(mux, "GET"+base+"/snapshots/{snapshotId}/file", func(ctx context.Context, r *http.Request) (contract.GitFile, error) {
		q := r.URL.Query()
		v, err := s.Code.GitSnapshotFile(ctx, r.PathValue("id"), q.Get("worktreeId"), q.Get("contextId"), r.PathValue("snapshotId"), q.Get("side"))
		return wire.GitFile(v), err
	})

	// Raw bytes stream outside the 10 s JSON wrapper; GET also serves HEAD.
	mux.HandleFunc("GET"+base+"/snapshots/{snapshotId}/raw", s.gitSnapshotRaw)

	serveGit(mux, "GET"+base+"/history", func(ctx context.Context, r *http.Request) (contract.GitHistoryPage, error) {
		q := r.URL.Query()
		limit, err := parseOptionalPositiveInt(q.Get("limit"))
		if err != nil {
			return contract.GitHistoryPage{}, err
		}
		v, err := s.Workspace.History(ctx, workspace.HistoryQuery{
			RepositoryID: r.PathValue("id"),
			WorktreeID:   q.Get("worktreeId"),
			Scope:        q.Get("scope"),
			Branch:       q.Get("branch"),
			Query:        q.Get("q"),
			Author:       q.Get("author"),
			Since:        q.Get("since"),
			Until:        q.Get("until"),
			Path:         q.Get("path"),
			Cursor:       q.Get("cursor"),
			Limit:        limit,
		})
		return wire.GitHistoryPage(v), err
	})

	serveGit(mux, "GET"+base+"/commits/{oid}", func(ctx context.Context, r *http.Request) (contract.GitCommitDetail, error) {
		v, err := s.Workspace.CommitDetail(ctx, r.PathValue("id"), r.PathValue("oid"), r.URL.Query().Get("parentOid"))
		return wire.GitCommitDetail(v), err
	})

	serveGit(mux, "GET"+base+"/operations", func(_ context.Context, r *http.Request) (contract.GitOperations, error) {
		q := r.URL.Query()
		limit, err := parseOptionalPositiveInt(q.Get("limit"))
		if err != nil {
			return contract.GitOperations{}, err
		}
		v, err := s.Workspace.ListGitOperations(r.PathValue("id"), q.Get("worktreeId"), limit)
		return wire.GitOperations(v), err
	})

	serveGit(mux, "GET"+base+"/operations/{operationId}", func(_ context.Context, r *http.Request) (contract.GitOperation, error) {
		v, err := s.Workspace.GetGitOperation(r.PathValue("id"), r.PathValue("operationId"))
		return wire.GitOperation(v), err
	})
}

func (s *Server) gitSnapshotRaw(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx, cancel := context.WithTimeout(r.Context(), rawLookupTimeout)
	t, err := s.Code.GitSnapshotRaw(ctx, r.PathValue("id"), q.Get("worktreeId"), q.Get("contextId"), r.PathValue("snapshotId"), q.Get("side"))
	cancel()
	sendTransferWithFailer(w, r, t, err, q.Get("download") == "1", gitFailure)
}
