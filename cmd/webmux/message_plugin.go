package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/localclient"
	"github.com/rainchan/webmux/internal/mcpstdio"
	"github.com/rainchan/webmux/internal/messageplugin"
	"github.com/rainchan/webmux/internal/messaging"
)

func messagePluginCommand(args []string, input io.Reader, output, diagnostics io.Writer) int {
	flags := flag.NewFlagSet("webmux message-plugin", flag.ContinueOnError)
	flags.SetOutput(diagnostics)
	socket := flags.String("socket", "", "local API socket PATH")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	credential := os.Getenv("WEBMUX_MESSAGES_CREDENTIAL")
	if *socket == "" || credential == "" {
		_, _ = fmt.Fprintln(diagnostics, "message-plugin requires a host-provided socket and credential")
		return 2
	}
	ctx := context.Background()
	client := localclient.New(localclient.UnixDialer{Path: *socket})
	defer client.Close()
	registry, err := (messageplugin.Client{API: client, Credential: credential}).Registry(ctx)
	if err == nil {
		err = mcpstdio.Serve(ctx, input, output, registry)
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostics, "message plugin transport closed")
		return 1
	}
	return 0
}

func messagingMCPServers(service *messaging.Service, executable string, socketPath func() string) func(context.Context, string, *agentprofile.Profile) []acp.MCPServer {
	return func(_ context.Context, id string, profile *agentprofile.Profile) []acp.MCPServer {
		socket := socketPath()
		if profile == nil || !mcpAvailable(socket, executable) {
			return nil
		}
		credential, err := service.Bind(id, profile.Title, profile.Name)
		if err != nil {
			return nil
		}
		return []acp.MCPServer{{Name: "webmux-messages-" + profile.Name, Command: executable, Args: []string{"message-plugin", "--socket", socket}, Env: []acp.MCPEnv{{Name: "WEBMUX_MESSAGES_CREDENTIAL", Value: credential}}}}
	}
}

func wakeMessagingAgent(chat *hybrid.Service) func(context.Context, messaging.Wake) error {
	var mu sync.Mutex
	attempts := map[string]uint32{}
	return func(ctx context.Context, task messaging.Wake) error {
		record, err := chat.Record(task.SessionID)
		if err != nil {
			return err
		}
		if record.State != hybrid.StateActive || record.Side != hybrid.SideACP {
			return hybrid.ErrWrongSide
		}
		mu.Lock()
		attempt := attempts[task.RequestID]
		mu.Unlock()
		_, err = chat.Prompt(ctx, task.SessionID, hybrid.PromptInput{ClientRequestID: fmt.Sprintf("%s-%d", task.RequestID, attempt), Mode: hybrid.ModeAuto, QueueKey: "messages:" + task.Channel, TitleSource: record.Title, Text: fmt.Sprintf("New channel messages in #%s. Use the webmux-messages tools to read the unacknowledged inbox, handle relevant requests, send any useful reply to the channel, and acknowledge only the messages you have handled. Repeat until the inbox is empty.", task.Channel)})
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			mu.Lock()
			if len(attempts) > 1000 {
				clear(attempts)
			}
			attempts[task.RequestID] = attempt + 1
			mu.Unlock()
		}
		return err
	}
}
