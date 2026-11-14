// Package transport owns the common controller request and response boundary.
package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/jsonutil"
)

// InputBodyBytes admits a 64-KiB terminal/chat input even when JSON escaped.
const InputBodyBytes = 8 * (64 << 10)

func Reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func Failure(w http.ResponseWriter, status int, err error) {
	message := err.Error()
	if status == http.StatusInternalServerError {
		slog.Error("request failed", "status", status, "error", err)
		message = http.StatusText(status)
	}
	Reply(w, status, contract.ErrorResponse{Error: message})
}

func Decode(w http.ResponseWriter, r *http.Request, value any) bool {
	limit := int64(65536)
	switch value.(type) {
	case *contract.SessionInputRequest, *contract.ChatQueueRequest:
		limit = InputBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		if _, sensitive := value.(*contract.TunnelSettingsUpdate); sensitive {
			err = errors.New("invalid tunnel settings JSON: use only known fields with the correct types")
		}
		Failure(w, http.StatusBadRequest, err)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		Failure(w, http.StatusBadRequest, errors.New("expected one JSON object"))
		return false
	}
	return true
}

func DecodeStrict(w http.ResponseWriter, r *http.Request, value any, limit int64, invalid error) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil || jsonutil.Decode(data, value) != nil {
		Failure(w, http.StatusBadRequest, invalid)
		return false
	}
	return true
}

func DecodeOptional(w http.ResponseWriter, r *http.Request, value any) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if err != nil {
		Failure(w, http.StatusBadRequest, err)
		return false
	}
	if len(data) == 0 {
		return true
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return Decode(w, r, value)
}

// BaseHeaders returns a deferred recovery shared by every transport.
func BaseHeaders(w http.ResponseWriter) func() {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	return func() {
		if p := recover(); p != nil {
			if p == http.ErrAbortHandler {
				panic(p)
			}
			slog.Error("request panic", "panic", p)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}
}

// Accept upgrades only after the host has checked authentication and Origin.
func Accept(w http.ResponseWriter, r *http.Request) hub.MessageConn {
	options := &websocket.AcceptOptions{InsecureSkipVerify: true, Subprotocols: []string{"webmux"}}
	if r.URL.Query().Get("compress") == "1" {
		options.CompressionMode = websocket.CompressionContextTakeover
	}
	conn, err := websocket.Accept(w, r, options)
	if err != nil {
		return nil
	}
	return hub.NewWebSocketConn(conn)
}
