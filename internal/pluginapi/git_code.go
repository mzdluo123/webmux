package pluginapi

import (
	"context"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	wire "github.com/rainchan/webmux/internal/pluginwire"
	"github.com/rainchan/webmux/internal/transport"
)

// gitCodeRoutes is registered only for the workspace+code combination.
func (s *Server) gitCodeRoutes(mux transport.Registrar) {
	s.gitCodeReadRoutes(mux)
	s.gitCodeTaskRoutes(mux)
	s.gitCodeStashRoutes(mux)
	s.gitCodeRevisionRoutes(mux)
}
func (s *Server) gitCodeStashRoutes(mux transport.Registrar) {
	const repoBase = " /api/repositories/{id}/git"
	serveGit(mux, "GET"+repoBase+"/stashes/{oid}/diff", func(ctx context.Context, r *http.Request) (contract.GitDiff, error) {
		q := r.URL.Query()
		untracked, err := parseOptionalBool(q.Get("untracked"))
		if err != nil {
			return contract.GitDiff{}, err
		}
		v, err := s.Code.GitStashDiff(
			ctx,
			r.PathValue("id"),
			q.Get("worktreeId"),
			q.Get("contextId"),
			r.PathValue("oid"),
			q.Get("path"),
			untracked,
		)
		return wire.GitDiff(v), err
	})
}
func (s *Server) gitCodeRevisionRoutes(mux transport.Registrar) {
	const gitBase = " /api/repositories/{id}/git"
	serveGit(mux, "GET"+gitBase+"/revision-diff", func(ctx context.Context, r *http.Request) (contract.GitDiff, error) {
		q := r.URL.Query()
		v, err := s.Code.GitRevisionDiff(
			ctx,
			r.PathValue("id"),
			q.Get("worktreeId"),
			q.Get("contextId"),
			q.Get("baseOid"),
			q.Get("path"),
		)
		return wire.GitDiff(v), err
	})
}
