package main

import (
	"os"

	snapcompact "github.com/mzdluo123/snapcompact-go"

	agyhistory "github.com/rainchan/webmux/internal/agent/agy/history"
	claudehistory "github.com/rainchan/webmux/internal/agent/claudecode/history"
	codexhistory "github.com/rainchan/webmux/internal/agent/codex/history"
	devinhistory "github.com/rainchan/webmux/internal/agent/devin/history"
	kimihistory "github.com/rainchan/webmux/internal/agent/kimi/history"
	omphistory "github.com/rainchan/webmux/internal/agent/omp/history"
	opencodehistory "github.com/rainchan/webmux/internal/agent/opencode/history"
	qoderhistory "github.com/rainchan/webmux/internal/agent/qoder/history"
	qodercnhistory "github.com/rainchan/webmux/internal/agent/qodercn/history"
	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
)

// Native history capabilities are independent of the selected executable/ACP
// registry: an import-only installation still has every supported reader.
type nativeHistoryCapability struct {
	read     historysource.ReadFunc
	discover historyimport.DiscoverFunc
	root     func(string, func(string) string) string
}

func nativeHistoryCapabilities() map[string]nativeHistoryCapability {
	return map[string]nativeHistoryCapability{
		"agy":        {agyhistory.Read, agyhistory.Discover, agyhistory.Root},
		"claudecode": {claudehistory.Read, claudehistory.Discover, claudehistory.Root},
		"codex":      {codexhistory.Read, codexhistory.Discover, codexhistory.Root},
		"devin":      {devinhistory.Read, devinhistory.Discover, devinhistory.Root},
		"kimi":       {kimihistory.Read, kimihistory.Discover, kimihistory.Root},
		"omp":        {omphistory.Read, omphistory.Discover, omphistory.Root},
		"opencode":   {opencodehistory.Read, opencodehistory.Discover, opencodehistory.Root},
		"qoder":      {qoderhistory.Read, qoderhistory.Discover, qoderhistory.Root},
		"qodercn":    {qodercnhistory.Read, qodercnhistory.Discover, qodercnhistory.Root},
	}
}

func nativeHistoryRoot(capabilities map[string]nativeHistoryCapability, agent string, env map[string]string) string {
	capability, ok := capabilities[agent]
	if !ok {
		return ""
	}
	getenv := func(key string) string {
		if value, exists := env[key]; exists {
			return value
		}
		return os.Getenv(key)
	}
	home := getenv("HOME")
	if _, explicit := env["HOME"]; !explicit && home == "" {
		home, _ = os.UserHomeDir()
	}
	return capability.root(home, getenv)
}

func (p *featureServices) openNativeHistory() {
	capabilities := nativeHistoryCapabilities()
	readers := make(map[string]historysource.ReadFunc, len(capabilities))
	discoverers := make(map[string]historyimport.DiscoverFunc, len(capabilities))
	roots := make(map[string]string, len(capabilities))
	for agent, capability := range capabilities {
		readers[agent], discoverers[agent] = capability.read, capability.discover
		roots[agent] = nativeHistoryRoot(capabilities, agent, nil)
	}
	p.nativeRoot = func(agent string, env map[string]string) string {
		return nativeHistoryRoot(capabilities, agent, env)
	}
	p.nativeHistory = historyimport.New(historyimport.Options{Store: p.conversations, Readers: readers, Discoverers: discoverers, Roots: roots})
	p.historyProjection = historyprojection.New(historyprojection.Options{Source: p.nativeHistory, Preprocess: historyprojection.DefaultPreprocessConfig(), Render: snapcompact.DefaultRenderConfig()})
}
