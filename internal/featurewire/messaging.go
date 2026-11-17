package featurewire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/messaging"
)

func MessagingChannel(item messaging.Channel) contract.MessagingChannel {
	return contract.MessagingChannel{ID: item.ID, Name: item.Name, Topic: item.Topic, LatestText: item.LatestText, LatestAt: item.LatestAt, MessageCount: item.MessageCount, MemberCount: item.MemberCount}
}

func MessagingChannels(items []messaging.Channel) []contract.MessagingChannel {
	out := make([]contract.MessagingChannel, 0, len(items))
	for _, item := range items {
		out = append(out, MessagingChannel(item))
	}
	return out
}

func MessagingMessage(item messaging.Message) contract.MessagingMessage {
	return contract.MessagingMessage{ID: item.ID, AgentID: item.AgentID, AgentName: item.AgentName, Text: item.Text, CreatedAt: item.CreatedAt}
}

func MessagingMessages(items []messaging.Message) []contract.MessagingMessage {
	out := make([]contract.MessagingMessage, 0, len(items))
	for _, item := range items {
		out = append(out, MessagingMessage(item))
	}
	return out
}

func MessagingIdentity(item messaging.Identity) contract.MessagingIdentity {
	return contract.MessagingIdentity{ID: item.ID, Name: item.Name, Profile: item.Profile}
}
func MessagingMember(item messaging.Membership) contract.MessagingMember {
	return contract.MessagingMember{MessagingIdentity: MessagingIdentity(item.Identity), Channel: item.Channel, Cursor: item.Cursor}
}
func MessagingMembers(items []messaging.Membership) []contract.MessagingMember {
	out := make([]contract.MessagingMember, 0, len(items))
	for _, item := range items {
		out = append(out, MessagingMember(item))
	}
	return out
}
func MessagingInbox(item messaging.Inbox) contract.MessagingInbox {
	out := contract.MessagingInbox{Messages: make([]contract.MessagingInboxItem, 0, len(item.Messages)), Through: item.Through}
	for _, m := range item.Messages {
		out.Messages = append(out.Messages, contract.MessagingInboxItem{MessagingMessage: MessagingMessage(m), Channel: m.Channel})
	}
	return out
}
