package archtest

import (
	"strings"
	"testing"
)

// webmux is the layer manifest. Every package under internal/ appears here;
// see docs/architecture.md for what each layer may depend on. Exceptions
// are listed with their reason and are visible in review.
var webmux = manifest{
	module: "github.com/rainchan/webmux",
	layers: map[string]layer{
		// HTTP and WebSocket transport.
		"internal/api":         controller,
		"internal/hub":         controller,
		"internal/mcphttp":     controller,
		"internal/wire":        controller,
		"internal/transport":   controller,
		"internal/featureapi":  controller,
		"internal/featurehub":  controller,
		"internal/featurewire": controller,

		// Business rules, state, filesystem and processes.
		"internal/session":           service,
		"internal/sessioncreate":     service,
		"internal/hybridresume":      service,
		"internal/hybrid":            service,
		"internal/handoff":           service,
		"internal/historyimport":     service,
		"internal/historyprojection": service,
		"internal/workspace":         service,
		"internal/settings":          service,
		"internal/launch":            service,
		"internal/agentcheck":        service,
		"internal/agenttrust":        service,
		"internal/push":              service,
		"internal/codeview":          service,
		"internal/files":             service,
		"internal/worktreefs":        service,
		"internal/forward":           service,
		"internal/artifacts":         service,
		"internal/tunnelsettings":    service,
		"internal/tailscale":         service,
		"internal/tailscalesettings": service,
		"internal/messaging":         service,
		"internal/voice":             service,
		"internal/voicesettings":     service,
		"internal/p2p":               service,
		"internal/p2psettings":       service,
		"internal/relaysetting":      service,

		// Wire contracts and infrastructure every layer may use.
		"internal/codexrollout":  shared,
		"internal/historysource": shared,
		"internal/acp":           shared,
		"internal/builtin":       shared,
		"internal/streamproto":   shared,
		"internal/nativerpc":     shared,
		// The shell guide served at /llms.txt and the {{webmux}} brief.
		"internal/agentguide":   shared,
		"internal/agentcatalog": shared,
		"internal/agentprofile": shared,
		// Agent adapters and the registry that lists them.
		"internal/agentprofile/agents":      shared,
		"internal/agent/copilot":            shared,
		"internal/agentprofile/trustconfig": shared,
		"internal/agent/agy":                shared,
		"internal/agent/claudecode":         shared,
		"internal/agent/codex":              shared,
		"internal/agent/codex/history":      shared,
		"internal/agent/claudecode/history": shared,
		"internal/agent/omp/history":        shared,
		"internal/agent/opencode/history":   shared,
		"internal/agent/devin/history":      shared,
		"internal/agent/agy/history":        shared,
		"internal/agent/kimi/history":       shared,
		"internal/agent/qoder/history":      shared,
		"internal/agent/qodercn/history":    shared,
		"internal/agent/devin":              shared,
		"internal/agent/kimi":               shared,
		"internal/agent/omp":                shared,
		"internal/agent/opencode":           shared,
		"internal/agent/qoder":              shared,
		"internal/agent/qodercn":            shared,
		"internal/contract":                 shared,
		"internal/config":                   shared,
		"internal/runtime":                  shared,
		"internal/hoststat":                 shared,
		"internal/jsonutil":                 shared,
		"internal/mcpprotocol":              shared,
		"internal/statefile":                shared,
		"internal/settingsstore":            shared,
		"internal/fetch":                    shared,
		"internal/update":                   shared,
		"internal/pathutil":                 shared,
		"internal/attentionkind":            shared,
		"internal/procpeek":                 shared,
		"internal/vtpreview":                shared,
		"internal/wsl":                      shared,

		// Programs that run outside the server and talk to it over the wire.
		"internal/client":         client,
		"internal/localclient":    client,
		"internal/sessionobserve": client,
		"internal/threadread":     client,
		"internal/messageplugin":  client,
		"internal/mcpstdio":       client,
		"internal/mcpcontrol":     client,
		"internal/attach":         client,
		"internal/clicmd":         client,
		"internal/tray":           client,
		"internal/tray/forward":   client,
		"internal/tray/traysys":   client,
		"internal/tray/winui":     client,
		"internal/wslclient":      client,
		"internal/tunnel":         client,
		"internal/service":        client,
		"internal/relay":          client,

		"internal/archtest":        testOnly,
		"internal/agent/agenttest": testOnly,
		"internal/docguard":        testOnly,
	},
	outbound: map[string]string{
		"internal/agentcheck": "queries read-only public npm, PyPI, and GitHub release endpoints on explicit update check requests",
		"internal/push":       "delivers Web Push messages to browser push services",
		"internal/p2p":        "queries local UPnP IGD router XML and SOAP port-mapping control endpoints",
		"internal/relay":      "authenticates to trusted Relay control HTTPS and assigned WSS connectors; serves streams only to the startup-fixed loopback listener",
	},
	pure: map[string]string{
		"internal/push.Validate":    "validates subscription strings without service state or I/O, before the controller checks Origin",
		"internal/hybrid.FormValue": "decodes one form answer value of a chat request body without service state or I/O",
	},
	contract: "internal/contract",
	contractAliases: map[string]string{
		"github.com/mzdluo123/webmux/protocol/relay": "published standard-library-only child module; only types explicitly aliased by internal/contract cross the host JSON boundary",
	},
	wireScope: []string{"internal/api", "internal/wire", "internal/hub", "internal/transport", "internal/featureapi", "internal/featurehub", "internal/featurewire"},
	// The controller's JSON boundary: their values are checked where they
	// are called.
	sinks: map[string]sink{
		"internal/api.reply":                 {arg: 2},
		"internal/api.decode":                {arg: 2},
		"internal/api.decodeOptional":        {arg: 2},
		"internal/api.decodeStrict":          {arg: 2},
		"internal/featureapi.reply":          {arg: 2},
		"internal/featureapi.decode":         {arg: 2},
		"internal/featureapi.decodeOptional": {arg: 2},
		"internal/featureapi.decodeStrict":   {arg: 2},
		"internal/featureapi.serveCode":      {typeArgs: []int{0}},
		"internal/featureapi.serveGit":       {typeArgs: []int{0}},
		"internal/featureapi.serve":          {typeArgs: []int{0}},
		"internal/featureapi.serveIn":        {typeArgs: []int{0, 1}},
		"internal/featureapi.serveInDone":    {typeArgs: []int{0}},
		"internal/transport.Reply":           {arg: 2},
		"internal/transport.Decode":          {arg: 2},
		"internal/transport.DecodeOptional":  {arg: 2},
		"internal/transport.DecodeStrict":    {arg: 2},
		"internal/api.serve":                 {typeArgs: []int{0}},
		"internal/api.serveIn":               {typeArgs: []int{0, 1}},
		"internal/api.serveInDone":           {typeArgs: []int{0}},
		"internal/hub.jsonFrame":             {arg: 1},
	},
	encoders: map[string]string{
		"internal/push.payload":              "Web Push message body, read by the service worker",
		"internal/push.testPayload":          "Web Push test message body",
		"internal/push.vapidHeader":          "VAPID JWT header and claims sent to push services",
		"internal/codeview.version":          "content digest of a snapshot, never sent as JSON",
		"internal/p2p.encodeControlJSON":     "encodes internal WebRTC DataChannel control handshake and RTT ping/pong frames",
		"internal/p2p.encodeRPCResponseJSON": "encodes internal WebRTC DataChannel RPC response metadata envelopes",
		"internal/historyimport.hashValue":   "hashes frozen source and projection identities, never client JSON",
		"internal/handoff.configFingerprint": "hashes launch argv/env without retaining credentials or emitting client JSON",
		"internal/handoff.jsonBytes":         "calculates internal handoff budget costs, never client-visible wire values",
	},
	bodyReaders: map[string]string{
		"internal/transport.Decode":                   "reads JSON bodies with a size limit and unknown fields rejected",
		"internal/transport.DecodeOptional":           "reads an optional bounded JSON body before centralized decoding",
		"internal/transport.DecodeStrict":             "reads bounded JSON bodies that also reject duplicate and case-folded keys",
		"internal/mcphttp.readBody":                   "bounds one MCP JSON message to 64 KiB and ten seconds before strict contract DTO/framing validation",
		"internal/api.decodeCreate":                   "reads the session create JSON body like decode, answering its failure as a create failure",
		"internal/featureapi.receiveUpload":           "streams one multipart file upload within the upload size limit",
		"internal/featureapi.(*Server).redeemForward": "reads the bounded gateway ticket form after host authentication",
	},
}

func TestArchitecture(t *testing.T) {
	p, err := load("../..", webmux)
	if err != nil {
		t.Fatal(err)
	}
	if violations := p.check(); len(violations) > 0 {
		lines := make([]string, len(violations))
		for i, v := range violations {
			lines[i] = v.String()
		}
		t.Errorf("%d architecture violations (docs/architecture.md):\n%s", len(violations), strings.Join(lines, "\n"))
	}
}
