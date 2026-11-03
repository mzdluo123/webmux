package tray

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// Exit codes of `webmux-tray firewall`.
const (
	FirewallDone     = 0 // the rule is in the requested state
	FirewallFailed   = 1 // netsh failed or the rule did not change
	FirewallUsage    = 2 // invalid arguments
	FirewallDeclined = 3 // the user declined the administrator prompt
)

// ErrElevationDeclined is returned by Firewall.Elevate when the user
// declines the administrator prompt.
var ErrElevationDeclined = errors.New("administrator prompt declined")

// ParseFirewall parses the arguments after `firewall`: exactly `allow` or
// `remove`.
func ParseFirewall(args []string) (allow bool, err error) {
	if len(args) != 1 {
		return false, errors.New("firewall takes one argument: allow or remove")
	}
	switch args[0] {
	case "allow":
		return true, nil
	case "remove":
		return false, nil
	}
	return false, errors.New("firewall takes allow or remove")
}

// FirewallRule is the inbound Windows Defender Firewall rule that lets
// other devices reach the tray's forwarding listeners. It belongs to one
// tray executable: the name carries a digest of its path, so installs in
// different directories never share or remove each other's rule, and a
// silent self-update in place keeps it.
type FirewallRule struct {
	Exe string
}

// Name is the rule's display name.
func (r FirewallRule) Name() string {
	sum := sha256.Sum256([]byte(strings.ToLower(r.Exe)))
	return "WebMux tray forwarding " + hex.EncodeToString(sum[:6])
}

// ShowArgs are the netsh arguments that succeed only when the rule exists.
// They need no administrator rights.
func (r FirewallRule) ShowArgs() []string {
	return []string{"advfirewall", "firewall", "show", "rule", "name=" + r.Name()}
}

// AddArgs add the rule: inbound TCP for the tray executable on private and
// domain networks only, never public ones.
func (r FirewallRule) AddArgs() []string {
	return []string{
		"advfirewall", "firewall", "add", "rule",
		"name=" + r.Name(),
		"dir=in", "action=allow", "protocol=TCP",
		"program=" + r.Exe,
		"profile=private,domain",
		"enable=yes",
		"description=Lets trusted devices reach WebMux through the tray's forwarding; added by the WebMux installer.",
	}
}

// DeleteArgs remove the rule.
func (r FirewallRule) DeleteArgs() []string {
	return []string{"advfirewall", "firewall", "delete", "rule", "name=" + r.Name()}
}

// Firewall brings Rule to the requested state through netsh. Exists runs
// netsh without administrator rights; Elevate runs it as administrator and
// returns its exit code, or ErrElevationDeclined.
type Firewall struct {
	Rule    FirewallRule
	Exists  func(args []string) (bool, error)
	Elevate func(args []string) (int, error)
}

// Apply adds (allow) or removes the rule and returns an exit code. It asks
// for administrator rights only when the rule is not already in that state.
func (f Firewall) Apply(allow bool) int {
	exists, err := f.Exists(f.Rule.ShowArgs())
	if err != nil {
		return FirewallFailed
	}
	if exists == allow {
		return FirewallDone
	}
	args := f.Rule.DeleteArgs()
	if allow {
		args = f.Rule.AddArgs()
	}
	code, err := f.Elevate(args)
	switch {
	case errors.Is(err, ErrElevationDeclined):
		return FirewallDeclined
	case err != nil || code != 0:
		return FirewallFailed
	}
	if exists, err = f.Exists(f.Rule.ShowArgs()); err != nil || exists != allow {
		return FirewallFailed
	}
	return FirewallDone
}
