// Command native-image-transport is an opt-in paid probe of WebMux's real ACP client.
// It does not start WebMux, attach to existing services, or change history handling.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/acp"
)

type observer struct {
	text  strings.Builder
	usage []acp.UsageUpdate
	tools int
}

func (o *observer) SessionUpdate(n acp.SessionNotification) {
	switch u := n.Update.(type) {
	case *acp.AgentMessageChunk:
		o.text.WriteString(u.Content.Text)
	case *acp.UsageUpdate:
		o.usage = append(o.usage, *u)
	case *acp.ToolCall:
		o.tools++
	}
}
func (*observer) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{}, errors.New("experiment forbids tools")
}
func (*observer) CreateElicitation(context.Context, acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	return acp.CreateElicitationResponse{}, errors.New("experiment forbids elicitation")
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func load(path string) []byte { b, err := os.ReadFile(path); must(err); return b }
func save(path string, value any) {
	b, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, append(b, '\n'), 0600))
}
func main() {
	directory := flag.String("directory", "", "synthetic generator output directory (required)")
	output := flag.String("output", "", "private evidence directory (required)")
	model := flag.String("model", "openai-codex/gpt-6.1-sol", "explicit real model")
	modes := flag.String("modes", "projected,image,hybrid", "comma-separated representations")
	timeout := flag.Duration("timeout", 120*time.Second, "hard lifetime per connector")
	flag.Parse()
	if *directory == "" || *output == "" {
		panic("-directory and -output are required")
	}
	must(os.MkdirAll(*output, 0700))
	var manifest struct {
		Expected map[string]any `json:"expected"`
		Image    struct {
			Pages []struct {
				File string `json:"file"`
			} `json:"pages"`
		} `json:"image"`
		Hybrid struct {
			Pages []struct {
				File string `json:"file"`
			} `json:"pages"`
		} `json:"hybrid"`
	}
	must(json.Unmarshal(load(filepath.Join(*directory, "manifest.json")), &manifest))
	for _, mode := range strings.Split(*modes, ",") {
		if mode != "raw" && mode != "projected" && mode != "image" && mode != "hybrid" {
			panic("unknown mode")
		}
		func() {
			temp, err := os.MkdirTemp("", "webmux-image-acp-")
			must(err)
			defer func() { must(os.RemoveAll(temp)) }()
			cwd := filepath.Join(temp, "cwd")
			must(os.Mkdir(cwd, 0700))
			config := filepath.Join(temp, "config.yml")
			must(os.WriteFile(config, []byte("{}\n"), 0600))
			args := []string{"acp", "--cwd", cwd, "--session-dir", filepath.Join(temp, "sessions"), "--config", config, "--model", *model, "--no-session", "--no-title", "--no-tools", "--no-extensions", "--no-skills", "--no-rules", "--no-context-files", "--no-lsp", "--offline", "--no-judge", "--no-spotlight", "--thinking", "off", "--max-cost", "1", "--max-time", fmt.Sprint(int(timeout.Seconds()))}
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, "omp", args...)
			cmd.Dir = cwd
			stdout, err := cmd.StdoutPipe()
			must(err)
			stdin, err := cmd.StdinPipe()
			must(err)
			stderr, err := os.OpenFile(filepath.Join(*output, mode+"-stderr.txt"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			must(err)
			defer func() { must(stderr.Close()) }()
			cmd.Stderr = stderr
			wire, err := os.OpenFile(filepath.Join(*output, mode+"-stdout.jsonl"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			must(err)
			defer func() { must(wire.Close()) }()
			start := time.Now()
			must(cmd.Start())
			o := &observer{}
			conn := acp.NewConn(io.TeeReader(stdout, wire), stdin, acp.Options{Handler: o})
			// Prompt cancellation waits for the agent; close as well on the hard deadline.
			go func() { <-ctx.Done(); _ = conn.Close() }()
			init, runErr := conn.Initialize(ctx, acp.Implementation{Name: "webmux-image-experiment", Version: "1"}, nil)
			var session acp.NewSessionResponse
			if runErr == nil {
				session, runErr = conn.NewSession(ctx, acp.NewSessionRequest{Cwd: cwd})
			}
			task := string(load(filepath.Join(*directory, "task.txt")))
			if mode != "image" {
				task += "\n\nHISTORY:\n" + string(load(filepath.Join(*directory, mode+".txt")))
			}
			blocks := []acp.ContentBlock{acp.TextBlock(task)}
			hashes := map[string]string{}
			pages := manifest.Image.Pages
			if mode == "hybrid" {
				pages = manifest.Hybrid.Pages
			}
			if mode == "image" || mode == "hybrid" {
				if runErr == nil && !init.AgentCapabilities.PromptCapabilities.Image {
					runErr = errors.New("connector does not advertise image prompts")
				}
				for _, page := range pages {
					b := load(filepath.Join(*directory, page.File))
					hash := sha256.Sum256(b)
					hashes[page.File] = hex.EncodeToString(hash[:])
					blocks = append(blocks, acp.ImageBlock(b, "image/png", ""))
				}
			}
			var response acp.PromptResponse
			if runErr == nil {
				response, runErr = conn.Prompt(ctx, acp.PromptRequest{SessionID: session.SessionID, Prompt: blocks})
			}
			elapsed := time.Since(start).Seconds()
			_ = conn.Close()
			_ = cmd.Process.Kill()
			<-conn.Done()
			_ = cmd.Wait()
			// Preserve omp's extra per-turn usage from the captured response.
			// The standard ACP response type intentionally does not expose it.
			var turnUsage json.RawMessage
			scanner := bufio.NewScanner(bytes.NewReader(load(filepath.Join(*output, mode+"-stdout.jsonl"))))
			scanner.Buffer(make([]byte, 4096), acp.MaxLineBytes)
			for scanner.Scan() {
				var event struct {
					Result struct {
						StopReason string          `json:"stopReason"`
						Usage      json.RawMessage `json:"usage"`
					} `json:"result"`
				}
				if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Result.StopReason != "" {
					turnUsage = event.Result.Usage
				}
			}
			must(scanner.Err())
			answerText := o.text.String()
			answer := map[string]any{}
			left, right := strings.Index(answerText, "{"), strings.LastIndex(answerText, "}")
			if left >= 0 && right >= left {
				_ = json.Unmarshal([]byte(answerText[left:right+1]), &answer)
			}
			correct := map[string]bool{}
			for key, expected := range manifest.Expected {
				if key == "middle_marker" && mode == "raw" {
					expected = "JADE-120"
				}
				correct[key] = answer[key] == expected
			}
			errorText := ""
			if runErr != nil {
				errorText = runErr.Error()
			}
			report := map[string]any{"mode": mode, "requested_model": *model, "command": append([]string{"omp"}, args...), "initialize": init, "session": session, "response": response, "error": errorText, "wall_seconds": elapsed, "answer": answer, "answer_text": answerText, "correct_fields": correct, "usage_updates": o.usage, "tool_calls": o.tools, "png_sha256": hashes, "prompt_bytes": len(task), "images": len(hashes), "transport": "WebMux internal/acp → real omp acp over private stdio; no HTTP service", "cost_note": "ACP usage/cost only; missing token categories are unknown, not zero; cost is connector estimate, not invoice"}
			report["turn_usage"] = turnUsage
			save(filepath.Join(*output, mode+"-report.json"), report)
			b, err := json.Marshal(report)
			must(err)
			fmt.Println(string(b))
		}()
	}
}
