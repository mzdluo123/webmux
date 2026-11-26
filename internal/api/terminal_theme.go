package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
)

func (s *Server) terminalTheme(r *http.Request, req contract.TerminalThemeNotification) error {
	if req.Theme != "light" && req.Theme != "dark" {
		return &badRequest{errors.New("theme 只能是 light 或 dark")}
	}
	s.Manager.ReportColorScheme(context.WithoutCancel(r.Context()), req.Theme == "dark")
	return nil
}
