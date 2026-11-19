package traysys

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

// Exit codes of `webmux-tray firewall`.
const (
	FirewallDone     = 0 // the rule is in the requested state
	FirewallFailed   = 1 // netsh failed or the rule did not change
	FirewallUsage    = 2 // invalid arguments
	FirewallDeclined = 3 // the user declined the administrator prompt
)

// ErrElevationDeclined is returned when the user declines the administrator prompt.
var ErrElevationDeclined = errors.New("administrator prompt declined")

// FirewallRequest holds parsed arguments of `webmux-tray firewall`.
type FirewallRequest struct {
	Allow       bool
	ClearBlocks bool
	Elevated    bool
	// Program is the absolute path of the tray whose rule to remove, or ""
	// for this executable.
	Program string
}

// ParseFirewallRequest parses the arguments after `firewall`:
//   - `allow [--clear-blocks] [--elevated]`
//   - `remove [--elevated] [ABS_PATH]`
func ParseFirewallRequest(args []string) (FirewallRequest, error) {
	if len(args) == 0 {
		return FirewallRequest{}, errors.New("firewall takes allow [--clear-blocks] or remove [ABS_PATH]")
	}
	var req FirewallRequest
	switch args[0] {
	case "allow":
		req.Allow = true
		for _, a := range args[1:] {
			switch a {
			case "--clear-blocks":
				req.ClearBlocks = true
			case "--elevated":
				req.Elevated = true
			default:
				return FirewallRequest{}, errors.New("firewall allow accepts only --clear-blocks")
			}
		}
		return req, nil
	case "remove":
		for _, a := range args[1:] {
			switch {
			case a == "--elevated":
				req.Elevated = true
			case req.Program == "" && isAbsWindowsPath(a):
				req.Program = a
			default:
				return FirewallRequest{}, errors.New("firewall remove takes at most one optional absolute tray path")
			}
		}
		return req, nil
	default:
		return FirewallRequest{}, errors.New("firewall takes allow [--clear-blocks] or remove [ABS_PATH]")
	}
}

// ParseFirewall parses `allow` or `remove [ABS_PATH]` and rejects internal flags.
func ParseFirewall(args []string) (allow bool, program string, err error) {
	req, err := ParseFirewallRequest(args)
	if err != nil || req.ClearBlocks || req.Elevated {
		return false, "", errors.New("firewall takes allow, or remove with an optional absolute tray path")
	}
	return req.Allow, req.Program, nil
}

// isAbsWindowsPath reports a drive-absolute (C:\...) or UNC path.
func isAbsWindowsPath(p string) bool {
	if strings.HasPrefix(p, `\\`) {
		return len(p) > 2
	}
	return len(p) > 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') &&
		(p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z')
}

// FirewallRule is the inbound Windows Defender Firewall rule that lets
// trusted devices reach the tray's forwarding listeners. Each rule name
// carries a digest of its executable path so distinct directories maintain
// independent rules while in-place updates keep the same rule.
type FirewallRule struct {
	Exe string
}

// Name returns the rule's display name.
func (r FirewallRule) Name() string {
	sum := sha256.Sum256([]byte(strings.ToLower(normalizeWinPath(r.Exe))))
	return "WebMux tray forwarding " + hex.EncodeToString(sum[:6])
}

// ShowArgs returns the netsh arguments querying rules with this display name.
func (r FirewallRule) ShowArgs() []string {
	return []string{"advfirewall", "firewall", "show", "rule", "name=" + r.Name()}
}

// AddArgs returns the netsh arguments adding the inbound TCP allow rule for
// private and domain networks.
func (r FirewallRule) AddArgs() []string {
	return []string{
		"advfirewall", "firewall", "add", "rule",
		"name=" + r.Name(),
		"dir=in", "action=allow", "protocol=TCP",
		"program=" + r.Exe,
		"profile=private,domain",
		"enable=yes",
		"description=Lets trusted devices reach WebMux through the tray's forwarding; added by WebMux.",
	}
}

// DeleteArgs returns the netsh arguments deleting rules with this display name.
func (r FirewallRule) DeleteArgs() []string {
	return []string{"advfirewall", "firewall", "delete", "rule", "name=" + r.Name()}
}

// DeleteProgramRuleArgs targets inbound rules with this name and executable.
func (r FirewallRule) DeleteProgramRuleArgs(name string, anyProtocol bool) []string {
	args := []string{"advfirewall", "firewall", "delete", "rule", "name=" + name, "program=" + r.Exe, "dir=in"}
	if !anyProtocol {
		args = append(args, "protocol=TCP")
	}
	return args
}

// ProfileMask is a bitmask of Windows Defender Firewall profiles.
type ProfileMask uint8

const (
	ProfileDomain ProfileMask = 1 << iota
	ProfilePrivate
	ProfilePublic
	ProfileAll = ProfileDomain | ProfilePrivate | ProfilePublic
)

// FirewallProfilePolicy describes profile-level firewall enforcement.
type FirewallProfilePolicy struct {
	Disabled        bool // EnableFirewall == 0
	BlockAllInbound bool // EnableFirewall != 0 && DoNotAllowExceptions == 1
}

// FirewallProfiles holds effective settings of Domain, Private and Public profiles.
type FirewallProfiles struct {
	Domain  FirewallProfilePolicy
	Private FirewallProfilePolicy
	Public  FirewallProfilePolicy
}

// ParsedFirewallRule holds fields decoded from a Windows Registry rule string.
type ParsedFirewallRule struct {
	Name     string
	Active   bool
	Dir      string
	Action   string
	Protocol string // "" for Any, "6" for TCP, "17" for UDP, ...
	Profiles ProfileMask
	App      string
	Narrowed bool // restricts port, address, service, package, interface or auth
}

// ParseFirewallRuleString parses an [MS-GPFAS] serialized rule string such as
// "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App=...|Name=...|".
func ParseFirewallRuleString(raw string) (ParsedFirewallRule, bool) {
	ver, rest, ok := strings.Cut(raw, "|")
	if !ok || len(ver) < 4 || (ver[0] != 'v' && ver[0] != 'V') || strings.IndexByte(ver[1:], '.') < 0 {
		return ParsedFirewallRule{}, false
	}
	var out ParsedFirewallRule
	hasProfile := false
	for seg := range strings.SplitSeq(rest, "|") {
		if seg == "" {
			continue
		}
		k, v, hasEq := strings.Cut(seg, "=")
		if !hasEq {
			continue
		}
		switch strings.ToLower(k) {
		case "name":
			out.Name = v
		case "active":
			out.Active = strings.EqualFold(v, "true")
		case "dir":
			out.Dir = strings.ToLower(v)
		case "action":
			out.Action = strings.ToLower(v)
		case "protocol":
			p := strings.ToLower(strings.TrimSpace(v))
			switch p {
			case "", "any", "256":
				out.Protocol = ""
			case "tcp":
				out.Protocol = "6"
			default:
				out.Protocol = p
			}
		case "app":
			out.App = v
		case "profile":
			hasProfile = true
			for tok := range strings.SplitSeq(v, ",") {
				switch strings.ToLower(strings.TrimSpace(tok)) {
				case "domain":
					out.Profiles |= ProfileDomain
				case "private":
					out.Profiles |= ProfilePrivate
				case "public":
					out.Profiles |= ProfilePublic
				case "all", "any":
					out.Profiles |= ProfileAll
				}
			}
		case "lport", "lport2_10", "lport2_20",
			"rport", "rport2_10", "rport2_20",
			"la4", "ra4", "la6", "ra6",
			"svc", "pkg", "if", "iftype",
			"luauth", "rmauth", "rmachine", "ruser", "luser":
			if !strings.EqualFold(v, "any") && v != "*" {
				out.Narrowed = true
			}
		}
	}
	if !hasProfile {
		out.Profiles = ProfileAll
	}
	return out, true
}

// FirewallStatus reports the effective read-only firewall state for one tray executable.
type FirewallStatus struct {
	// RulePresent reports whether any local rule has Rule.Name().
	RulePresent bool
	// RuleValid reports whether the single local rule named Rule.Name() is
	// enabled and allows unrestricted inbound TCP to Rule.Exe on Domain and
	// Private profiles.
	RuleValid bool
	// AllowedPrivateDomain reports whether inbound TCP to Rule.Exe is allowed
	// on both Domain and Private profiles by Rule.Name() or another rule.
	AllowedPrivateDomain bool
	// LocalBlockNames lists sorted unique display names of active local inbound
	// Block rules targeting Rule.Exe on Domain or Private profiles (excluding
	// Rule.Name(), which DeleteArgs removes directly).
	LocalBlockNames   []string
	anyProtocolBlocks map[string]bool
	// ManualLocalBlock reports a block that netsh cannot select without
	// possibly deleting an unrelated rule.
	ManualLocalBlock bool
	// PolicyBlocked reports an active policy/MDM inbound Block rule targeting
	// Rule.Exe on Domain or Private profiles.
	PolicyBlocked bool
	Profiles      FirewallProfiles
}

// EvaluateFirewall derives FirewallStatus from local and policy rule strings.
func EvaluateFirewall(rule FirewallRule, localRules, policyRules []string, profiles FirewallProfiles, sameExe func(app, exe string) bool) FirewallStatus {
	if sameExe == nil {
		sameExe = sameWindowsPath
	}
	wantName := rule.Name()
	const privateDomain = ProfileDomain | ProfilePrivate
	st := FirewallStatus{Profiles: profiles}

	namedTotal, namedValid := 0, 0
	type selector struct {
		block           string
		anyProtocol     bool
		other, otherTCP bool
	}
	selectors := map[string]selector{}
	for _, raw := range localRules {
		p, ok := ParseFirewallRuleString(raw)
		if !ok {
			continue
		}
		isNamed := strings.EqualFold(p.Name, wantName)
		if isNamed {
			namedTotal++
			if p.Active && p.Dir == "in" && p.Action == "allow" && (p.Protocol == "" || p.Protocol == "6") &&
				p.Profiles == privateDomain && p.App != "" && sameExe(p.App, rule.Exe) && !p.Narrowed {
				namedValid++
			}
		}
		if p.Dir != "in" || p.App == "" || !sameExe(p.App, rule.Exe) {
			continue
		}
		tcpRule := p.Protocol == "6"
		coversTCP := p.Active && (p.Protocol == "" || tcpRule)
		if coversTCP && p.Action == "allow" && !p.Narrowed && p.Profiles&privateDomain == privateDomain {
			st.AllowedPrivateDomain = true
		}
		if isNamed {
			continue
		}
		key := strings.ToLower(p.Name)
		s := selectors[key]
		if p.Action == "block" && (p.Protocol == "" || tcpRule) && p.Profiles&privateDomain != 0 {
			if !p.Active {
				continue
			}
			if trimmed := strings.TrimSpace(key); trimmed == "" || trimmed == "all" {
				st.ManualLocalBlock = true // netsh rejects blank names and reserves name=all as a wildcard.
			} else {
				s.block = p.Name
				s.anyProtocol = s.anyProtocol || !tcpRule
			}
		} else {
			s.other = true
			s.otherTCP = s.otherTCP || tcpRule
		}
		selectors[key] = s
	}
	st.RulePresent = namedTotal > 0
	st.RuleValid = namedTotal == 1 && namedValid == 1
	if st.RuleValid {
		st.AllowedPrivateDomain = true
	}
	for _, s := range selectors {
		if s.block == "" {
			continue
		}
		if s.otherTCP || (s.anyProtocol && s.other) {
			st.ManualLocalBlock = true
		} else {
			st.LocalBlockNames = append(st.LocalBlockNames, s.block)
			if s.anyProtocol {
				if st.anyProtocolBlocks == nil {
					st.anyProtocolBlocks = make(map[string]bool)
				}
				st.anyProtocolBlocks[s.block] = true
			}
		}
	}
	sort.Strings(st.LocalBlockNames)

	for _, raw := range policyRules {
		p, ok := ParseFirewallRuleString(raw)
		if !ok || !p.Active || p.Dir != "in" || (p.Protocol != "" && p.Protocol != "6") || p.App == "" || !sameExe(p.App, rule.Exe) {
			continue
		}
		switch p.Action {
		case "allow":
			if !p.Narrowed && p.Profiles&privateDomain == privateDomain {
				st.AllowedPrivateDomain = true
			}
		case "block":
			if p.Profiles&privateDomain != 0 {
				st.PolicyBlocked = true
			}
		}
	}
	return st
}

func normalizeWinPath(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "/", `\`)
	for len(p) > 3 && strings.HasSuffix(p, `\`) {
		p = strings.TrimSuffix(p, `\`)
	}
	return p
}

func sameWindowsPath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(normalizeWinPath(a), normalizeWinPath(b))
}

// Firewall brings Rule to the requested state through netsh. Inspect reads the
// current rule state without administrator rights; Run executes one or more
// netsh argv commands with administrator rights (or returns ErrElevationDeclined).
type Firewall struct {
	Rule    FirewallRule
	Inspect func() (FirewallStatus, error)
	Run     func(cmds [][]string) (int, error)
}

// Apply adds/repairs (allow) or removes the rule and returns an exit code.
func (f Firewall) Apply(allow, clearBlocks bool) int {
	st, err := f.Inspect()
	if err != nil {
		return FirewallFailed
	}
	if !allow {
		if !st.RulePresent {
			return FirewallDone
		}
		if code := f.exec([][]string{f.Rule.DeleteArgs()}); code != FirewallDone {
			return code
		}
		if st, err = f.Inspect(); err != nil || st.RulePresent {
			return FirewallFailed
		}
		return FirewallDone
	}

	if clearBlocks && st.ManualLocalBlock {
		return FirewallFailed
	}
	needUnblock := clearBlocks && len(st.LocalBlockNames) > 0
	if st.RuleValid && !needUnblock {
		return FirewallDone
	}
	var cmds [][]string
	if st.RulePresent {
		cmds = append(cmds, f.Rule.DeleteArgs())
	}
	if needUnblock {
		for _, name := range st.LocalBlockNames {
			if !strings.EqualFold(name, f.Rule.Name()) {
				cmds = append(cmds, f.Rule.DeleteProgramRuleArgs(name, st.anyProtocolBlocks[name]))
			}
		}
	}
	cmds = append(cmds, f.Rule.AddArgs())
	if code := f.exec(cmds); code != FirewallDone {
		return code
	}
	st, err = f.Inspect()
	if err != nil || !st.RuleValid {
		return FirewallFailed
	}
	if clearBlocks && (len(st.LocalBlockNames) > 0 || st.ManualLocalBlock) {
		return FirewallFailed
	}
	return FirewallDone
}

func (f Firewall) exec(cmds [][]string) int {
	code, err := f.Run(cmds)
	switch {
	case errors.Is(err, ErrElevationDeclined):
		return FirewallDeclined
	case err != nil || code != 0:
		return FirewallFailed
	}
	return FirewallDone
}
