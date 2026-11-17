package featurewire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/messaging"
)

func MessagingChannel(item messaging.Channel) contract.MessagingChannel {
	return contract.MessagingChannel{ID: item.ID, Name: item.Name, Topic: item.Topic}
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
