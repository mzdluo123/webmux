package featurewire

import (
	"strconv"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historysource"
)

// ConversationActivity preserves native text verbatim and pages on UTF-8 byte
// boundaries. Unknown runtime provenance stays empty rather than being inferred.
func ConversationActivity(record historysource.Record, conversationID string, position int, hash string, offset, maxBytes int) (contract.ConversationActivity, error) {
	var out contract.ConversationActivity
	if offset < 0 || offset > len(record.Text) || maxBytes < 4 || maxBytes > conversation.MaxActivityBytes || (offset < len(record.Text) && !utf8.RuneStart(record.Text[offset])) {
		return out, conversation.ErrInvalid
	}
	end := min(len(record.Text), offset+maxBytes)
	for end < len(record.Text) && !utf8.RuneStart(record.Text[end]) {
		end--
	}
	exportable := record.Exportable()
	text := record.Text[offset:end]
	reason := ""
	if !exportable {
		text, reason = "", "non_content"
		end = len(record.Text)
	}
	out = contract.ConversationActivity{
		Exportable: exportable, ExclusionReason: reason,
		ActivityID: record.ID, ConversationID: conversationID,
		Position: strconv.Itoa(position), RunOrdinal: "0",
		Kind: record.Kind, Role: record.Role, Status: record.Status,
		Text: text, Source: "native", ProviderItemID: record.NativeID,
		ByteLength: strconv.Itoa(len(record.Text)), NextTextOffset: strconv.Itoa(end),
		Truncated: end < len(record.Text), SourceHash: hash,
		ParentID: record.ParentID, NativeID: record.NativeID,
		CallID: record.CallID, NativeCallID: record.NativeCallID, ToolName: record.ToolName,
		Attachments: each(record.Attachments, func(v historysource.Attachment) contract.ConversationAttachment {
			return contract.ConversationAttachment{Name: v.Name, MimeType: v.MimeType, Reference: v.Reference, Size: v.Size}
		}),
	}
	return out, nil
}

func ConversationContext(c conversation.Conversation) contract.ConversationContext {
	return contract.ConversationContext{ID: c.ID, Cwd: c.Cwd, RepositoryID: c.RepositoryID, WorktreeID: c.WorktreeID}
}
