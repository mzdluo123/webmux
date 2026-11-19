package hybrid

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/agent/kimi"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
)

func TestReapplyHandledConfigErrorWithoutAdvertisedOptionOrInPlaceMutation(t *testing.T) {
	cmd, err := agents.Default().Parse([]string{kimi.Name})
	if err != nil {
		t.Fatal(err)
	}
	a := &actor{
		id:  "abcdefgh",
		cmd: cmd,
		svc: &Service{log: slog.New(slog.DiscardHandler)},
	}

	newHandledLink := func(t *testing.T) *acpLink {
		t.Helper()
		clientIn, agentOut := io.Pipe()
		agentIn, clientOut := io.Pipe()
		conn := acp.NewConn(clientIn, clientOut, acp.Options{})
		t.Cleanup(func() {
			_ = conn.Close()
			_ = agentOut.Close()
			_ = agentIn.Close()
		})
		go func() {
			br := bufio.NewReader(agentIn)
			for {
				line, readErr := br.ReadBytes('\n')
				if readErr != nil {
					return
				}
				var msg struct {
					ID json.RawMessage `json:"id"`
				}
				if json.Unmarshal(line, &msg) != nil {
					return
				}
				resp := `{"jsonrpc":"2.0","id":` + string(msg.ID) + `,"error":{"code":-32603,"message":"internal error","data":{"details":"Already in plan mode"}}}` + "\n"
				_, _ = io.WriteString(agentOut, resp)
			}
		}()
		return &acpLink{conn: conn, sessionID: "session_test"}
	}

	want := []wanted{{id: "mode", value: "plan"}}

	// Empty options slice: ConfigErrorHandled returns true without indexing options[-1].
	gotEmpty, notes := a.reapply(newHandledLink(t), want, nil)
	if len(gotEmpty) != 0 || len(notes) != 0 {
		t.Fatalf("empty options reapply = (%+v, %v), want empty without notes", gotEmpty, notes)
	}

	// Non-empty options without target ID: preserves existing entries without mutating or panicking.
	withoutTarget := []acp.ConfigOption{{
		ID:      "model",
		Name:    "Model",
		Type:    acp.ConfigSelect,
		Current: acp.ConfigValue{ID: "kimi-k2"},
	}}
	gotMissing, notes := a.reapply(newHandledLink(t), want, withoutTarget)
	if len(notes) != 0 || len(gotMissing) != 1 || gotMissing[0].Current.ID != "kimi-k2" {
		t.Fatalf("missing target reapply = (%+v, %v)", gotMissing, notes)
	}

	// Options containing target ID: updates returned copy without mutating caller's backing slice.
	original := []acp.ConfigOption{{
		ID:      "mode",
		Name:    "Mode",
		Type:    acp.ConfigSelect,
		Current: acp.ConfigValue{ID: "default"},
	}}
	gotUpdated, notes := a.reapply(newHandledLink(t), want, original)
	if len(notes) != 0 || len(gotUpdated) != 1 || gotUpdated[0].Current.ID != "plan" {
		t.Fatalf("present target reapply = (%+v, %v)", gotUpdated, notes)
	}
	if original[0].Current.ID != "default" {
		t.Fatalf("reapply mutated caller slice in place: %+v", original)
	}
}
