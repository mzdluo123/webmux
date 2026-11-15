//go:build linux || darwin

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/launch"
)

func TestLaunchOnlyCLIAddsRunnableConfig(t *testing.T) {
	f := newFixture(t, "[features]\nenabled = [\"launch\"]\n")
	f.start(t, nil, true, "--config", f.config)
	if stdout, stderr, code := f.cli(t, "launch-config", "add", "Plain", "--", "/bin/sh", "-c", "printf CLI_LAUNCH_ONLY_OK"); code != 0 {
		t.Fatalf("add terminal config: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code := f.cli(t, "run", "--launch-config", "Plain", "--follow", "--strip")
	if code != 0 || stdout != "CLI_LAUNCH_ONLY_OK" {
		t.Fatalf("run saved config: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestLaunchConfigCLIEditPreservesOwnedFields(t *testing.T) {
	for _, row := range []struct {
		name, features string
	}{
		{"disabled owners", `["launch"]`},
		{"selected owners", `["launch", "files", "chat", "notifications"]`},
	} {
		t.Run(row.name, func(t *testing.T) {
			f := newFixture(t, "[features]\nenabled = "+row.features+"\n")
			store := launch.Open(f.state)
			original, err := store.Create(launch.Input{
				Name: "Stored", Cmd: []string{"omp"}, Env: map[string]string{"KEEP": "secret"}, Hybrid: true,
				Attention: &launch.AttentionSettings{Mode: "urgent", ProgramStatus: true, AgentProtocol: true, Notify: true, Bell: true, MinTurnSeconds: 7, DoneDelaySeconds: 3},
			})
			store.Close()
			if err != nil {
				t.Fatal(err)
			}
			f.start(t, nil, true, "--config", f.config)
			client := f.local(t)
			read := func() contract.LaunchConfig {
				t.Helper()
				var value contract.LaunchConfig
				if err := client.Do(context.Background(), http.MethodGet, "/api/launch-configs/"+original.ID, nil, &value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			before := read()
			stdout, stderr, code := f.cli(t, "launch-config", "edit", "Stored", "--rename", "Renamed", "--preview-lines", "5", "--env", "EXTRA=value")
			if code != 0 {
				t.Fatalf("edit config: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			after := read()
			if after.Name != "Renamed" || after.Env["KEEP"] != "secret" || after.Env["EXTRA"] != "value" || after.Preview == nil || after.Preview.Lines != 5 {
				t.Fatalf("requested fields or retained environment: %+v", after)
			}
			if after.Chat != before.Chat || !reflect.DeepEqual(after.Attention, before.Attention) {
				prior, _ := json.Marshal(before)
				current, _ := json.Marshal(after)
				t.Fatalf("unmodified feature options changed: before=%s after=%s", prior, current)
			}
			if row.name == "disabled owners" {
				if _, stderr, code := f.cli(t, "launch-config", "edit", "Renamed", "--no-chat"); code != contract.ExitError {
					t.Fatalf("disabled chat mutation: exit=%d stderr=%q", code, stderr)
				}
				if current := read(); !current.Chat || !reflect.DeepEqual(current.Attention, before.Attention) {
					t.Fatal("rejected feature mutation changed stored options")
				}
			}
			if strings.Contains(stdout+stderr, "secret") {
				t.Fatal("CLI exposed a stored environment value")
			}
		})
	}
}
