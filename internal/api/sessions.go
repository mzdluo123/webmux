package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/wire"
)

var requestID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// SessionExtensions binds optional lifecycle and projection behavior at the
// composition root. Plain sessions never need a plugin receiver.
type SessionExtensions interface {
	ListArchived(context.Context, []contract.SessionState) ([]contract.SessionInfo, error)
	Info(context.Context, string) (contract.SessionInfo, error)
	Kind(string) (bool, error)
	Input(context.Context, string, session.InputRequest) (session.InputResult, error)
	Interrupt(context.Context, string, string, string) error
	Forget(context.Context, string) error
	CreatedLocation(*sessioncreate.Location) *contract.Worktree
	CreateFailure(http.ResponseWriter, error)
	CommandFailure(http.ResponseWriter, error)
}

func (s *Server) commandFailure(w http.ResponseWriter, err error) {
	var bad *badRequest
	if errors.As(err, &bad) || errors.Is(err, session.ErrInputFailed) || errors.Is(err, session.ErrWrongInputDestination) {
		sessionCommandFailure(w, err)
		return
	}
	if s.Sessions != nil {
		s.Sessions.CommandFailure(w, err)
		return
	}
	sessionCommandFailure(w, err)
}
func (s *Server) sessionRoutes(mux *http.ServeMux) {
	serve(mux, "GET /api/sessions", http.StatusOK, s.commandFailure, s.list)
	mux.HandleFunc("POST /api/sessions", s.create)
	serve(mux, "GET /api/sessions/{id}", http.StatusOK, s.commandFailure, func(r *http.Request) (contract.SessionInfo, error) { return s.sessionInfo(r.PathValue("id")) })
	mux.HandleFunc("DELETE /api/sessions/{id}", s.delete)
	serveIn(mux, "POST /api/sessions/{id}/input", http.StatusOK, s.commandFailure, s.input)
	serveInDone(mux, "POST /api/sessions/{id}/interrupt", s.commandFailure, s.interrupt)
	serveIn(mux, "POST /api/sessions/{id}/resize", http.StatusOK, s.commandFailure, s.resize)
}

var sessionStates = map[string][]contract.SessionState{
	"active": {contract.SessionStateActive}, "closed": {contract.SessionStateClosed}, "interrupted": {contract.SessionStateInterrupted},
	"archived": {contract.SessionStateClosed, contract.SessionStateInterrupted}, "all": {contract.SessionStateActive, contract.SessionStateClosed, contract.SessionStateInterrupted},
}

func (s *Server) list(r *http.Request) ([]contract.SessionInfo, error) {
	want := map[contract.SessionState]bool{}
	values := r.URL.Query()["state"]
	if len(values) == 0 {
		values = []string{"active"}
	}
	for _, v := range values {
		states, ok := sessionStates[v]
		if !ok {
			return []contract.SessionInfo{}, &badRequest{errors.New("state 只能是 active、closed、interrupted、archived 或 all")}
		}
		for _, state := range states {
			want[state] = true
		}
	}
	out := []contract.SessionInfo{}
	if want[contract.SessionStateActive] {
		for _, info := range s.Manager.List() {
			snapshot := s.sessionSnapshot(info)
			if snapshot.State == contract.SessionStateActive {
				out = append(out, snapshot)
			}
		}
	}
	if s.Sessions != nil && (want[contract.SessionStateClosed] || want[contract.SessionStateInterrupted]) {
		states := []contract.SessionState{}
		for _, state := range []contract.SessionState{contract.SessionStateClosed, contract.SessionStateInterrupted} {
			if want[state] {
				states = append(states, state)
			}
		}
		archived, err := s.Sessions.ListArchived(r.Context(), states)
		if err != nil {
			return []contract.SessionInfo{}, err
		}
		out = append(out, archived...)
	}
	return out, nil
}
func (s *Server) sessionInfo(id string) (contract.SessionInfo, error) {
	if s.Sessions != nil {
		return s.Sessions.Info(context.Background(), id)
	}
	item, err := s.Manager.Get(id)
	if err != nil {
		return contract.SessionInfo{}, err
	}
	return s.sessionSnapshot(item.Info()), nil
}
func (s *Server) sessionKind(id string) (bool, error) {
	if s.Sessions != nil {
		return s.Sessions.Kind(id)
	}
	_, err := s.Manager.Get(id)
	return false, err
}
func (s *Server) inputLabel(r *http.Request) string {
	if caller := s.callerSession(r); caller != "" {
		return "script · " + caller
	}
	return "script"
}
func (s *Server) input(r *http.Request, req contract.SessionInputRequest) (contract.SessionInputResult, error) {
	if !requestID.MatchString(req.ClientRequestID) {
		return contract.SessionInputResult{}, &badRequest{errors.New("clientRequestId 必须是 8 到 64 个字母、数字、- 或 _")}
	}
	if _, ok := wire.InputKeys(req.Keys); !ok {
		return contract.SessionInputResult{}, &badRequest{errors.New("未知按键")}
	}
	if req.Text == "" && len(req.Keys) == 0 && len(req.Attachments) == 0 && !req.Submit {
		return contract.SessionInputResult{}, &badRequest{errors.New("没有要输入的内容")}
	}
	id := r.PathValue("id")
	chat, err := s.sessionKind(id)
	if err != nil {
		return contract.SessionInputResult{}, err
	}
	in := wire.SessionInput(req, s.inputLabel(r))
	var result session.InputResult
	if chat {
		result, err = s.Sessions.Input(r.Context(), id, in)
	} else {
		item, getErr := s.Manager.Get(id)
		if getErr != nil {
			return contract.SessionInputResult{}, getErr
		}
		result, err = item.Input(r.Context(), in)
	}
	return wire.SessionInputResult(result), err
}
func (s *Server) resize(r *http.Request, req contract.SessionResizeRequest) (contract.SessionInfo, error) {
	item, err := s.Manager.Get(r.PathValue("id"))
	if err != nil {
		return contract.SessionInfo{}, err
	}
	if err := item.ResizeScript(wire.ScriptResize(req, s.inputLabel(r))); err != nil {
		return contract.SessionInfo{}, err
	}
	return s.sessionSnapshot(item.Info()), nil
}
func (s *Server) interrupt(r *http.Request, req contract.SessionCommandRequest) error {
	if !requestID.MatchString(req.ClientRequestID) {
		return &badRequest{errors.New("clientRequestId 必须是 8 到 64 个字母、数字、- 或 _")}
	}
	id := r.PathValue("id")
	chat, err := s.sessionKind(id)
	if err != nil {
		return err
	}
	if chat {
		return s.Sessions.Interrupt(r.Context(), id, req.ClientRequestID, s.inputLabel(r))
	}
	item, err := s.Manager.Get(id)
	if err != nil {
		return err
	}
	return item.Interrupt(r.Context(), req.ClientRequestID, s.inputLabel(r))
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateSessionRequest
	if err := decodeCreate(w, r, &req); err != nil {
		reply(w, http.StatusBadRequest, contract.CreateSessionFailure{Error: err.Error(), Step: sessioncreate.StepPrecheck})
		return
	}
	if err := s.Manager.ValidateLabels(req.Labels); err != nil {
		reply(w, http.StatusBadRequest, contract.CreateSessionFailure{Error: err.Error(), Step: sessioncreate.StepPrecheck})
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), sessioncreate.Timeout)
	defer cancel()
	result, err := s.Create.Create(ctx, wire.CreateInput(req))
	if err != nil {
		if s.Sessions != nil {
			s.Sessions.CreateFailure(w, err)
		} else {
			createFailure(w, err)
		}
		return
	}
	id := result.Session.Info().ID
	info, err := s.adopt(r, id, req.Labels)
	if err != nil {
		info = result.Session.Info()
	}
	out := wire.CreateResult(result, info)
	out.Session = s.sessionSnapshot(info)
	if s.Sessions != nil {
		out.Worktree = s.Sessions.CreatedLocation(result.Location)
	}
	reply(w, http.StatusCreated, out)
}
func decodeCreate(w http.ResponseWriter, r *http.Request, req *contract.CreateSessionRequest) error {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, chatBodyBytes))
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(req); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) > 0 {
		return errors.New("expected one JSON object")
	}
	return nil
}
func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	query := r.URL.Query()
	var err error
	switch {
	case query.Get("forget") == "1":
		err = s.forget(r.Context(), id)
	case query.Get("remove") == "1":
		err = s.Manager.Remove(id)
	default:
		err = s.Manager.Close(id)
	}
	if err != nil {
		s.commandFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) forget(ctx context.Context, id string) error {
	if s.Sessions != nil {
		return s.Sessions.Forget(ctx, id)
	}
	return s.Manager.Remove(id)
}
