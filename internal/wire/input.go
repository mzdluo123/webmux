package wire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

func InputKeys(names []string) (keys [][]byte, ok bool) {
	for _, name := range names {
		key, known := contract.InputKeys[name]
		if !known {
			return nil, false
		}
		keys = append(keys, []byte(key))
	}
	return keys, true
}
func SessionInput(r contract.SessionInputRequest, label string) session.InputRequest {
	keys, _ := InputKeys(r.Keys)
	in := session.InputRequest{ClientRequestID: r.ClientRequestID, Label: label, Text: r.Text, Submit: r.Submit, Keys: keys, Mode: r.Mode}
	for _, at := range r.Attachments {
		in.Attachments = append(in.Attachments, session.InputAttachment{Path: at.Path})
	}
	return in
}
func SessionInputResult(r session.InputResult) contract.SessionInputResult {
	out := contract.SessionInputResult{View: contract.ViewTerminal}
	if r.Side == "acp" {
		out.View = contract.ViewChat
		queued := r.Prompt.Queued
		out.Queued = &queued
		out.ItemID = r.Prompt.ItemID
		out.Position = r.Prompt.Position
		out.TurnID = r.Prompt.TurnID
		out.PromptID = r.Prompt.PromptID
	}
	return out
}
