package main

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/messaging"
)

func TestMessagePluginDescriptorsForEveryHybridAgent(t *testing.T) {
	s := messaging.Open(t.TempDir())
	defer s.Close()
	socket := t.TempDir() + "/api.sock"
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	callback := messagingMCPServers(s, executable, func() string { return socket })
	for _, agent := range agents.Default().Agents() {
		t.Run(agent.Profile().Name, func(t *testing.T) {
			profile := agent.Profile()
			servers := callback(context.Background(), "session-"+profile.Name, profile)
			if len(servers) != 1 || servers[0].Name != "webmux-messages-"+profile.Name || len(servers[0].Env) != 1 {
				t.Fatalf("descriptor: %+v", servers)
			}
			identity, err := s.Authenticate(servers[0].Env[0].Value)
			if err != nil || identity.Profile != profile.Name || identity.ID != "session:session-"+profile.Name {
				t.Fatalf("scope: %+v %v", identity, err)
			}
		})
	}
}
