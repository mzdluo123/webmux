package statefile

// ID is a registered persistence capability, not a caller-supplied path.
type ID string

const (
	Launch             ID = "launch"
	Repositories       ID = "repositories"
	Projects           ID = "projects"
	ProjectClones      ID = "project-clones"
	Tunnel             ID = "tunnel"
	TailscaleSettings  ID = "tailscale-settings"
	VoiceSettings      ID = "voice-settings"
	P2PSettings        ID = "p2p-settings"
	Relay              ID = "relay"
	TailscaleOwnership ID = "tailscale-ownership"
	Push               ID = "push"
	Runtime            ID = "runtime"
	Settings           ID = "settings"
	Hybrid             ID = "hybrid"
	AgentOptions       ID = "agent-options"
	GitOperations      ID = "git-operations"
	GitTaskBaselines   ID = "git-task-baselines"
	GitWorkbenchState  ID = "git-workbench-state"
)

// Descriptor is the single runtime source of storage paths and policy.
// Domain validation remains with the owning service.
type Descriptor struct {
	ID                              ID
	Path, Owner, Failure, Sensitive string
	Version                         int
	VersionKey                      string
	MaxBytes                        int64
	Web                             bool
	Legacy                          string
	// MinReadVersion admits older schemas for domain-owned migrations.
	// Zero means only Version is readable; writes always require Version.
	MinReadVersion int
}

func (d Descriptor) LockPath() string { return d.Path + ".lock" }

var catalog = []Descriptor{
	{Launch, "launch-configs.json", "internal/launch", "unavailable", "command/env", 2, "v", 4 << 20, true, "", 0},
	{ProjectClones, "project-clones.json", "internal/workspace", "unavailable", "paths/urls", 1, "version", 4 << 20, true, "", 0},
	{Projects, "projects.json", "internal/workspace", "unavailable", "paths", 1, "version", 4 << 20, true, "", 0},
	{Repositories, "repositories.json", "internal/workspace", "unavailable", "paths", 1, "version", 4 << 20, true, "", 0},
	{GitOperations, "git-operations.json", "internal/workspace", "unavailable", "paths/urls", 1, "v", 2 << 20, true, "", 0},
	{GitTaskBaselines, "git-task-baselines.json", "internal/workspace", "unavailable", "paths", 1, "v", 2 << 20, true, "", 0},
	{GitWorkbenchState, "git-workbench-state.json", "internal/workspace", "unavailable", "paths/commit-draft", 1, "v", 2 << 20, true, "", 0},
	{Tunnel, "tunnel-settings.json", "internal/tunnelsettings", "fail_closed", "token", 1, "v", 64 << 10, true, "tunnel.json", 0},
	{TailscaleSettings, "tailscale-settings.json", "internal/tailscalesettings", "fail_closed", "none", 1, "v", 64 << 10, true, "", 0},
	{VoiceSettings, "voice-settings.json", "internal/voicesettings", "fail_closed", "env", 1, "v", 64 << 10, true, "", 0},
	{P2PSettings, "p2p-settings.json", "internal/p2psettings", "fail_closed", "none", 1, "v", 64 << 10, true, "", 0},
	{Relay, "relay-settings.json", "internal/relaysetting", "fail_closed", "Ed25519 identity/binding", 2, "v", 64 << 10, true, "", 1},
	{TailscaleOwnership, "tailscale-ownership.json", "cmd/webmux", "block_start", "target", 1, "v", 64 << 10, false, "tailscale.json", 0},
	{Push, "push.json", "internal/push", "unavailable", "vapid/subscriptions", 1, "v", 1 << 20, true, "", 0},
	{Runtime, "run/server.json", "internal/runtime", "local_api_unavailable", "fingerprint", 1, "v", 64 << 10, false, "", 0},
	{Settings, "settings.json", "internal/settings", "unavailable", "commands", 1, "v", 4 << 20, true, "", 0},
	{Hybrid, "hybrid-sessions.json", "internal/hybrid", "unavailable", "argv/cwd", 1, "v", 1 << 20, true, "", 0},
	{AgentOptions, "agent-options.json", "internal/hybrid", "unavailable", "none", 1, "v", 1 << 20, true, "", 0},
}

func Catalog() []Descriptor { return append([]Descriptor(nil), catalog...) }
func Describe(id ID) Descriptor {
	for _, d := range catalog {
		if d.ID == id {
			return d
		}
	}
	return Descriptor{}
}
