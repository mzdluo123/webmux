package historyprojection

import (
	"context"
	"encoding/json"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/historysource"
)

// CompactOMPDocument adapts the frozen, exportable records to OMP messages.
// The archive pass owns normalization, tool budgets and chronological layout.
func CompactOMPDocument(ctx context.Context, doc historysource.Document, options snapcompact.OMPOptions) (snapcompact.OMPArchive, error) {
	messages := make([]snapcompact.OMPMessage, 0, len(doc.Records))
	for _, record := range doc.Records {
		if err := ctx.Err(); err != nil {
			return snapcompact.OMPArchive{}, err
		}
		if !record.Exportable() {
			continue
		}
		callID := record.CallID
		if callID == "" {
			callID = "\x00" + record.ID
		}
		message := snapcompact.OMPMessage{Role: "user"}
		content := snapcompact.OMPContent{Type: "text", Text: record.Text}
		switch record.Kind {
		case "tool_arguments", "tool-call", "tool":
			message.Role = "assistant"
			content.Type, content.ID, content.Name = "toolCall", callID, record.ToolName
			if content.Name == "" {
				content.Name = "tool"
			}
			content.Arguments = json.RawMessage(record.Text)
		case "tool_result", "tool-result":
			message.Role = "toolResult"
		default:
			switch record.Role {
			case "assistant", "agent":
				message.Role = "assistant"
			case "tool":
				message.Role = "toolResult"
			}
		}
		if message.Role == "toolResult" {
			message.ToolCallID = callID
			message.IsError = record.Status == "failed" || record.Status == "error"
		}
		message.Content = []snapcompact.OMPContent{content}
		messages = append(messages, message)
	}
	return snapcompact.CompactOMP(ctx, messages, options)
}
