package mcpcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
	"github.com/rainchan/webmux/internal/mcpprotocol"
)

var requestID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func object(properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
}
func boolean() map[string]any      { return map[string]any{"type": "boolean"} }
func stringsArray() map[string]any { return map[string]any{"type": "array", "items": text()} }
func stringMap() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": text()}
}

func (c Client) actionTools() []mcpprotocol.Tool {
	create := createProperties()
	if !c.hasCapability("launch.configs") {
		delete(create, "configId")
		delete(create, "useInitInstruction")
	}
	if !c.hasCapability("workspace.manage") {
		delete(create, "workspace")
		delete(create, "newWorktree")
		delete(create, "useInitInstruction")
	}
	if !c.hasCapability("chat.acp") {
		create["view"] = desc(enumeration("terminal"), "Initial view; only terminal is available on this instance.")
		delete(create, "agentConfig")
		delete(create, "permissionMode")
	}
	if !c.hasCapability("files.upload") {
		delete(create, "attachments")
	}
	input := map[string]any{
		"clientRequestId": requestIDSchema(),
		"text":            desc(text(), "Terminal view: characters typed as one submission; add submit to include Enter. Chat view: the prompt text."),
		"submit":          desc(boolean(), "Terminal view only (the CLI's --enter): append the Enter key to text in the same submission. Text without submit stays on the input line without running. Ignored on the chat view."),
		"keys":            desc(stringsArray(), "Terminal view only (the CLI's --key): named keys sent after the text — backspace, ctrl-c, ctrl-d, down, enter, esc, left, right, tab, up. Rejected with wrong_view on the chat view."),
	}
	if c.hasCapability("chat.acp") {
		input["mode"] = desc(enumeration("auto", "steer", "refuse", "now"), "Chat view only (the CLI's --mode). "+
			"auto (default): use for an independent next task; after readiness and input validation, start only when no turn is running, the queue is unpaused and empty, and the agent connection is available; otherwise queue. "+
			"steer: use to supplement or correct the current task; requires a linked agent, a profile implementing Steerer, chat.Steering and a running turn. It does not cancel the turn or guarantee immediate processing. "+
			"After connection and view checks, missing support or no running turn returns steering_unavailable. "+
			"Only an adapter promptRequired race outcome puts the input at the queue head; startedNewTurn warns that the adapter started a turn WebMux does not track. "+
			"refuse: use when automation must not queue; after readiness and input validation, start under the same conditions as auto, otherwise return busy. "+
			"now: use only to explicitly stop and replace the current task; cancellation can interrupt work. Cancel a running turn and prioritize this input at the queue head, or send directly when no turn is running and the connection is available; with neither a running turn nor a connection, return busy.")
	}
	if c.hasCapability("files.upload") {
		input["attachments"] = desc(attachmentSchema(), "Chat view only (the CLI's --attach-file): uploaded file references ({path}) sent with the prompt.")
	}
	return []mcpprotocol.Tool{
		tool("session_launch", "Create a session, optionally using the instance's enabled launch/workspace/chat capabilities. Requires a stable clientRequestId; retry the identical request after a lost response. Creation is deduplicated by the server for ten minutes, in memory only. Partial failures retain created resources.", false, create, []string{"clientRequestId"}, c.launch),
		tool("session_send", "Send input through the current session view: typed text on the terminal view, a prompt on the chat view. "+
			"Read state before sending. Choose steer for a current-task supplement, auto for an independent next task, now for an explicit stop-and-replace, or refuse when queueing is unacceptable. "+
			"If the input is already queued, check its status and remove only your own item confirmed still queued before deciding whether to steer, to avoid duplicate delivery. This control MCP has no queue-removal tool; use an available CLI/API or ask the user to remove it. "+
			"Retry with the same clientRequestId and identical payload; use a new id for a new operation. "+
			"Chat receipts may contain a turnId or queued itemId; steer usually has neither turnId nor promptId. "+
			"A successful steer receipt does not prove understanding or completion; terminal acceptance does not prove execution or completion.", false, map[string]any{
			"sessionId": sessionIDSchema(), "request": desc(object(input, "clientRequestId", "text"), "Same shape as the body of POST /api/sessions/{id}/input."),
		}, []string{"sessionId", "request"}, c.send),
		tool("session_interrupt", "Interrupt the current chat turn or send the terminal interrupt key. Does not delete the session or its history. Reuse clientRequestId on retries.", false, map[string]any{
			"sessionId": sessionIDSchema(), "clientRequestId": requestIDSchema(),
		}, []string{"sessionId", "clientRequestId"}, c.interrupt),
	}
}
func sessionIDSchema() map[string]any {
	return desc(text(), "Session id, for example from session_list or session_launch.")
}
func requestIDSchema() map[string]any {
	return desc(text(), "Stable request id, 8-64 letters, digits, - or _. Reuse it with the identical payload when retrying a lost response; generate a new one for a new operation.")
}
func attachmentSchema() map[string]any {
	return map[string]any{"type": "array", "items": object(map[string]any{"path": text(), "name": text(), "mimeType": text(), "kind": text(), "size": integer(0, 1<<30)}, "path")}
}
func createProperties() map[string]any {
	return map[string]any{
		"clientRequestId":    requestIDSchema(),
		"configId":           desc(text(), "Saved launch configuration id from capabilities launchConfigs; mutually exclusive with cmd."),
		"cmd":                desc(stringsArray(), "Full argv for the process; mutually exclusive with configId."),
		"env":                desc(stringMap(), "Extra environment variables."),
		"cwd":                desc(text(), "Working directory."),
		"name":               desc(text(), "Display name."),
		"workspace":          desc(object(map[string]any{"repositoryId": text(), "worktreeId": text()}, "repositoryId", "worktreeId"), "Run in an existing worktree."),
		"newWorktree":        desc(object(map[string]any{"repositoryId": text(), "mode": enumeration("new", "existing"), "branch": text(), "base": text(), "path": text()}, "repositoryId", "mode"), "Create or reuse a worktree: repositoryId, mode new or existing, plus optional branch, base and path."),
		"labels":             desc(stringMap(), "Session labels."),
		"view":               desc(enumeration("terminal", "chat"), "Initial view; chat requires a chat-capable launch."),
		"prompt":             desc(text(), "Initial input after launch: a prompt on the chat view or a submitted line on the terminal view."),
		"useInitInstruction": desc(boolean(), "Send the repository's init instruction as the initial prompt."),
		"expandPrompt":       desc(boolean(), "Expand placeholders such as {{webmux}} in prompt."),
		"attachments":        desc(attachmentSchema(), "Uploaded file references ({path}) sent with the initial prompt; on the terminal view they are appended to the typed line as path text."),
		"agentConfig":        desc(stringMap(), "Per-agent option overrides, chosen from the agent's declared settings."),
		"permissionMode":     desc(enumeration("ask", "auto"), "How the agent's permission requests are handled."),
		"cols":               desc(integer(1, 1000), "Initial terminal width."),
		"rows":               desc(integer(1, 1000), "Initial terminal height."),
	}
}

func (c Client) launch(ctx context.Context, raw json.RawMessage) (any, error) {
	var req contract.CreateSessionRequest
	if decode(raw, &req) != nil || !requestID.MatchString(req.ClientRequestID) {
		return nil, errors.New("session_launch requires valid arguments and clientRequestId (8-64 letters, digits, - or _)")
	}
	var result contract.CreateSessionResult
	if err := c.API.DoBounded(ctx, http.MethodPost, "/api/sessions", req, responseLimit, &result); err != nil {
		var e *localclient.APIError
		if errors.As(err, &e) {
			var failure contract.CreateSessionFailure
			if json.Unmarshal(e.Body, &failure) == nil && failure.Step != "" {
				data, marshalErr := json.Marshal(failure)
				if marshalErr != nil {
					return nil, errors.New("cannot encode creation failure")
				}
				return nil, errors.New(string(data))
			}
		}
		return nil, apiError(err)
	}
	// Link lookup is advisory: a successful creation must never look failed just
	// because a second metadata read is unavailable.
	var details contract.SessionDetails
	_ = c.API.GetBounded(ctx, "/api/sessions/"+url.PathEscape(result.Session.ID)+"/details", responseLimit, &details)
	out := contract.MCPSessionLaunchResult{ClientRequestID: req.ClientRequestID, Session: result.Session, Worktree: result.Worktree, ConfigWarnings: result.ConfigWarnings, Link: details.Link, InitialInput: result.InitialInput}

	if result.InitialInput != nil {
		switch {
		case result.InitialInput.TurnID != "":
			out.Wait = &contract.MCPSessionWaitSelector{SessionID: result.Session.ID, TurnID: result.InitialInput.TurnID}
		case result.InitialInput.ItemID != "":
			out.Wait = &contract.MCPSessionWaitSelector{SessionID: result.Session.ID, QueueItemID: result.InitialInput.ItemID}
		}
	}

	return out, nil
}
func (c Client) send(ctx context.Context, raw json.RawMessage) (any, error) {
	var a contract.MCPSessionSendRequest
	if decode(raw, &a) != nil || !identity.MatchString(a.SessionID) || !requestID.MatchString(a.Request.ClientRequestID) {
		return nil, errors.New("invalid session_send arguments or clientRequestId")
	}
	var result contract.SessionInputResult
	if err := c.API.DoBounded(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(a.SessionID)+"/input", a.Request, responseLimit, &result); err != nil {
		return nil, apiError(err)
	}
	return contract.MCPSessionSendResult{ClientRequestID: a.Request.ClientRequestID, SessionID: a.SessionID, Receipt: result}, nil
}
func (c Client) interrupt(ctx context.Context, raw json.RawMessage) (any, error) {
	var a contract.MCPSessionInterruptRequest
	if decode(raw, &a) != nil || !identity.MatchString(a.SessionID) || !requestID.MatchString(a.ClientRequestID) {
		return nil, errors.New("invalid session_interrupt arguments or clientRequestId")
	}
	err := c.API.DoBounded(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(a.SessionID)+"/interrupt", contract.SessionCommandRequest{ClientRequestID: a.ClientRequestID}, responseLimit, nil)
	return contract.MCPSessionInterruptResult{SessionID: a.SessionID, ClientRequestID: a.ClientRequestID, Interrupted: err == nil}, apiError(err)
}
