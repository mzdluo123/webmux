package clicmd

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/localclient"
)

func TestMCPCancellationUnblocksIdleStdin(t *testing.T) {
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	var stdout, stderr bytes.Buffer
	e := Env{Stdin: input, Stdout: &stdout, Stderr: &stderr, Connect: func() (*localclient.Client, error) {
		return localclient.New(localclient.UnixDialer{Path: "/unused"}), nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- e.Run(ctx, mcpCommand, nil) }()
	cancel()
	select {
	case code := <-done:
		if code == 0 {
			t.Fatal("cancelled command reported normal EOF")
		}
	case <-time.After(time.Second):
		t.Fatal("idle MCP command did not stop while stdin remained open")
	}
}
