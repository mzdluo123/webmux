package assemble

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

func TestCoreOnlySessionHonorsTerminalStatusSettings(t *testing.T) {
	t.Setenv("WARP_CLI_AGENT_PROTOCOL_VERSION", "")
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			body := fmt.Sprintf("agent_protocol = %t\nbell_attention = %t\nprogram_status = %t\n[plugins]\nenabled = []\n", enabled, enabled, enabled)
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			c, err := config.Load(path, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.Validate(); err != nil {
				t.Fatal(err)
			}
			manager := session.NewManager(SessionOptions(c))
			creator := sessioncreate.New(manager, sessioncreate.Options{DefaultCwd: dir})
			t.Cleanup(func() {
				creator.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
				defer cancel()
				manager.Shutdown(ctx)
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			created, err := creator.Create(ctx, sessioncreate.Input{
				Cwd: dir,
				Cmd: []string{"/bin/sh", "-c", `printf 'PROTOCOL=<%s>\n' "$WARP_CLI_AGENT_PROTOCOL_VERSION"; printf '\033]7501;state=blocked:kind=permission\007SIGNAL_READY\n'; sleep 30`},
			})
			if err != nil {
				t.Fatal(err)
			}
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			var output string
			for {
				output = string(created.Session.Buffer())
				if strings.Contains(output, "SIGNAL_READY") {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatalf("terminal did not publish status signal: %q", output)
				case <-ticker.C:
				}
			}
			protocol := "PROTOCOL=<>"
			if enabled {
				protocol = "PROTOCOL=<1>"
			}
			if !strings.Contains(output, protocol) {
				t.Fatalf("agent protocol environment: want %q in %q", protocol, output)
			}
			if status := created.Session.Info().Status; (status == "waiting") != enabled {
				t.Fatalf("program status signal enabled=%t: status=%s", enabled, status)
			}
		})
	}
}
