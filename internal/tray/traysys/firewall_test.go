package traysys

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestParseFirewall(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		allow   bool
		program string
		ok      bool
	}{
		{[]string{"allow"}, true, "", true},
		{[]string{"remove"}, false, "", true},
		{[]string{"remove", `D:\Old\webmux-tray.exe`}, false, `D:\Old\webmux-tray.exe`, true},
		{[]string{"remove", `\\server\share\webmux-tray.exe`}, false, `\\server\share\webmux-tray.exe`, true},
		{[]string{"remove", `webmux-tray.exe`}, false, "", false},
		{[]string{"allow", `D:\Other\evil.exe`}, false, "", false},
		{nil, false, "", false},
		{[]string{"add"}, false, "", false},
	} {
		allow, program, err := ParseFirewall(tc.args)
		if (err == nil) != tc.ok || (tc.ok && (allow != tc.allow || program != tc.program)) {
			t.Errorf("ParseFirewall(%q) = %v, %q, %v", tc.args, allow, program, err)
		}
	}
}

func TestParseFirewallRequest(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want FirewallRequest
		ok   bool
	}{
		{[]string{"allow"}, FirewallRequest{Allow: true}, true},
		{[]string{"allow", "--clear-blocks"}, FirewallRequest{Allow: true, ClearBlocks: true}, true},
		{[]string{"allow", "--clear-blocks", "--elevated"}, FirewallRequest{Allow: true, ClearBlocks: true, Elevated: true}, true},
		{[]string{"remove", "--elevated", `C:\WebMux\webmux-tray.exe`}, FirewallRequest{Elevated: true, Program: `C:\WebMux\webmux-tray.exe`}, true},
		{[]string{"remove", "--clear-blocks"}, FirewallRequest{}, false},
		{[]string{"allow", "--clear-blocks", `C:\Other\app.exe`}, FirewallRequest{}, false},
		{[]string{"remove", `C:\A\a.exe`, `C:\B\b.exe`}, FirewallRequest{}, false},
	} {
		got, err := ParseFirewallRequest(tc.args)
		if (err == nil) != tc.ok || (tc.ok && got != tc.want) {
			t.Errorf("ParseFirewallRequest(%q) = %+v, %v", tc.args, got, err)
		}
	}
}

func TestFirewallRuleNamePerExecutable(t *testing.T) {
	a := FirewallRule{Exe: `C:\Users\u\AppData\Local\Programs\WebMux\webmux-tray.exe`}
	if got := (FirewallRule{Exe: strings.ToUpper(a.Exe)}).Name(); got != a.Name() {
		t.Errorf("case changed the name: %q vs %q", got, a.Name())
	}
	if got := (FirewallRule{Exe: strings.ReplaceAll(a.Exe, `\`, `/`)}).Name(); got != a.Name() {
		t.Errorf("slash changed the name: %q vs %q", got, a.Name())
	}
	if b := (FirewallRule{Exe: `D:\Tools\WebMux\webmux-tray.exe`}); b.Name() == a.Name() {
		t.Errorf("two paths share %q", a.Name())
	}
	if !strings.HasPrefix(a.Name(), "WebMux tray forwarding ") {
		t.Errorf("name %q", a.Name())
	}
}

func TestFirewallRuleAddArgs(t *testing.T) {
	r := FirewallRule{Exe: `C:\Program Files\Web Mux\webmux-tray.exe`}
	args := r.AddArgs()
	for _, want := range []string{"dir=in", "action=allow", "protocol=TCP", "profile=private,domain", "program=" + r.Exe, "name=" + r.Name()} {
		if !slices.Contains(args, want) {
			t.Errorf("AddArgs lacks %q: %q", want, args)
		}
	}
	for _, a := range args {
		if strings.HasPrefix(a, "profile=") && a != "profile=private,domain" {
			t.Errorf("AddArgs opens more than private and domain networks: %q", a)
		}
	}
}

func TestEvaluateFirewall(t *testing.T) {
	const exe = `C:\WebMux\webmux-tray.exe`
	rule := FirewallRule{Exe: exe}
	validRule := "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|Desc=WebMux|"

	t.Run("valid rule", func(t *testing.T) {
		st := EvaluateFirewall(rule, []string{validRule}, nil, FirewallProfiles{}, nil)
		if !st.RulePresent || !st.RuleValid || !st.AllowedPrivateDomain || len(st.LocalBlockNames) != 0 || st.PolicyBlocked {
			t.Fatalf("unexpected status: %+v", st)
		}
	})

	t.Run("any protocol includes TCP", func(t *testing.T) {
		raw := strings.Replace(validRule, "Protocol=6", "Protocol=256", 1)
		if st := EvaluateFirewall(rule, []string{raw}, nil, FirewallProfiles{}, nil); !st.RuleValid {
			t.Fatalf("expected valid rule: %+v", st)
		}
	})

	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"disabled", "v2.31|Action=Allow|Active=FALSE|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"},
		{"active omitted", "v2.31|Action=Allow|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"},
		{"action block", "v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"},
		{"outbound", "v2.31|Action=Allow|Active=TRUE|Dir=Out|Protocol=6|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"},
		{"udp only", "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=17|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"},
		{"missing domain", "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"},
		{"widened to public", "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|App=" + exe + "|Name=" + rule.Name() + "|"},
		{"other app", "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App=C:\\Other\\webmux-tray.exe|Name=" + rule.Name() + "|"},
		{"narrowed port", "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|LPort=8080|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"},
	} {
		t.Run("tampered/"+tc.name, func(t *testing.T) {
			st := EvaluateFirewall(rule, []string{tc.raw}, nil, FirewallProfiles{}, nil)
			if !st.RulePresent || st.RuleValid {
				t.Fatalf("expected present invalid rule, got %+v", st)
			}
		})
	}

	t.Run("duplicate named rules require cleanup", func(t *testing.T) {
		st := EvaluateFirewall(rule, []string{validRule, validRule}, nil, FirewallProfiles{}, nil)
		if !st.RulePresent || st.RuleValid {
			t.Fatalf("duplicate rules treated as valid: %+v", st)
		}
	})

	t.Run("local block rules vs public-only or other-app blocks", func(t *testing.T) {
		rules := []string{
			validRule,
			// Active inbound TCP/Any blocks for exe on Private/All:
			"v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=6|Profile=Private|App=" + strings.ToLower(exe) + "|Name=webmux-tray.exe|",
			"v2.31|Action=Block|Active=TRUE|Dir=In|App=" + exe + "|Name=WebMux|",
			// Ignored: Public-only block, UDP-only block, disabled block, generic port block without App, other executable:
			"v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=6|Profile=Public|App=" + exe + "|Name=PublicBlock|",
			"v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=17|Profile=Private|App=" + exe + "|Name=UDPBlock|",
			"v2.31|Action=Block|Active=FALSE|Dir=In|Protocol=6|Profile=Private|App=" + exe + "|Name=DisabledBlock|",
			"v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=6|LPort=445|Name=SMBBlock|",
			"v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=6|App=C:\\Other\\app.exe|Name=OtherApp|",
		}
		st := EvaluateFirewall(rule, rules, nil, FirewallProfiles{}, nil)
		wantBlocks := []string{"WebMux", "webmux-tray.exe"}
		if !reflect.DeepEqual(st.LocalBlockNames, wantBlocks) || st.ManualLocalBlock {
			t.Fatalf("blocks = %+v, want %v", st, wantBlocks)
		}
	})

	t.Run("policy allow and policy block", func(t *testing.T) {
		gpoAllow := "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|Profile=Domain,Private|App=" + exe + "|Name=CorpAllow|"
		st := EvaluateFirewall(rule, nil, []string{gpoAllow}, FirewallProfiles{}, nil)
		if st.RulePresent || st.RuleValid || !st.AllowedPrivateDomain || st.PolicyBlocked {
			t.Fatalf("gpo allow: %+v", st)
		}
		gpoBlock := "v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=6|Profile=Private|App=" + exe + "|Name=CorpBlock|"
		st = EvaluateFirewall(rule, []string{validRule}, []string{gpoBlock}, FirewallProfiles{}, nil)
		if !st.PolicyBlocked {
			t.Fatalf("expected PolicyBlocked: %+v", st)
		}
	})
}

type fakeNetsh struct {
	rules      []string
	inspectErr error
	batches    [][][]string
	code       int
	err        error
	noMutate   bool
}

func (n *fakeNetsh) firewall() Firewall {
	const exe = `C:\WebMux\webmux-tray.exe`
	rule := FirewallRule{Exe: exe}
	return Firewall{
		Rule: rule,
		Inspect: func() (FirewallStatus, error) {
			if n.inspectErr != nil {
				return FirewallStatus{}, n.inspectErr
			}
			return EvaluateFirewall(rule, n.rules, nil, FirewallProfiles{}, nil), nil
		},
		Run: func(cmds [][]string) (int, error) {
			n.batches = append(n.batches, cmds)
			if n.err != nil || n.code != 0 || n.noMutate {
				return n.code, n.err
			}
			for _, cmd := range cmds {
				switch cmd[2] {
				case "delete":
					targetName := strings.TrimPrefix(cmd[4], "name=")
					var filterDir, filterProto, filterProg string
					for _, arg := range cmd[5:] {
						switch {
						case strings.HasPrefix(arg, "dir="):
							filterDir = strings.ToLower(strings.TrimPrefix(arg, "dir="))
						case strings.HasPrefix(arg, "protocol="):
							if strings.EqualFold(strings.TrimPrefix(arg, "protocol="), "TCP") {
								filterProto = "6"
							}
						case strings.HasPrefix(arg, "program="):
							filterProg = strings.TrimPrefix(arg, "program=")
						}
					}
					filtered := n.rules[:0]
					for _, raw := range n.rules {
						p, ok := ParseFirewallRuleString(raw)
						if ok && strings.EqualFold(p.Name, targetName) &&
							(filterDir == "" || p.Dir == filterDir) &&
							(filterProto == "" || p.Protocol == filterProto) &&
							(filterProg == "" || sameWindowsPath(p.App, filterProg)) {
							continue
						}
						filtered = append(filtered, raw)
					}
					n.rules = filtered
				case "add":
					n.rules = append(n.rules, "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App="+exe+"|Name="+rule.Name()+"|")
				}
			}
			return 0, nil
		},
	}
}

func TestFirewallApply(t *testing.T) {
	const exe = `C:\WebMux\webmux-tray.exe`
	rule := FirewallRule{Exe: exe}
	validRule := "v2.31|Action=Allow|Active=TRUE|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"
	disabledRule := "v2.31|Action=Allow|Active=FALSE|Dir=In|Protocol=6|Profile=Domain|Profile=Private|App=" + exe + "|Name=" + rule.Name() + "|"
	localBlock := "v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=6|Profile=Private|App=" + exe + "|Name=webmux-tray.exe|"
	disabledLocalBlock := strings.Replace(localBlock, "Active=TRUE", "Active=FALSE", 1)
	udpBlock := strings.Replace(localBlock, "Protocol=6", "Protocol=17", 1)
	outboundBlock := strings.Replace(localBlock, "Dir=In", "Dir=Out", 1)
	unnamedBlock := "v2.31|Action=Block|Active=TRUE|Dir=In|Protocol=6|Profile=Private|App=" + exe + "|"
	blankBlock := strings.Replace(localBlock, "Name=webmux-tray.exe", "Name=   ", 1)
	collidingAllow := strings.Replace(localBlock, "Action=Block", "Action=Allow", 1)
	wildcardBlock := strings.Replace(localBlock, "Name=webmux-tray.exe", "Name=ALL", 1)

	for _, tc := range []struct {
		name        string
		netsh       fakeNetsh
		allow       bool
		clearBlocks bool
		want        int
		verbs       []string // verbs inside the single Run batch, or nil when no elevation
		wantRules   int      // optional post-Apply rule count check when > 0
	}{
		{name: "allow adds a missing rule", netsh: fakeNetsh{}, allow: true, want: FirewallDone, verbs: []string{"add"}},
		{name: "allow with valid rule present asks nothing", netsh: fakeNetsh{rules: []string{validRule}}, allow: true, want: FirewallDone},
		{name: "allow repairs disabled rule in one batch", netsh: fakeNetsh{rules: []string{disabledRule}}, allow: true, want: FirewallDone, verbs: []string{"delete", "add"}},
		{name: "allow clears local block and repairs in one batch", netsh: fakeNetsh{rules: []string{validRule, localBlock}}, allow: true, clearBlocks: true, want: FirewallDone, verbs: []string{"delete", "delete", "add"}},
		{
			name:        "allow clears active and disabled tcp blocks while keeping udp and outbound rules",
			netsh:       fakeNetsh{rules: []string{localBlock, disabledLocalBlock, udpBlock, outboundBlock}},
			allow:       true,
			clearBlocks: true,
			want:        FirewallDone,
			verbs:       []string{"delete", "add"},
			wantRules:   3,
		},
		{name: "allow unnamed block fails without elevation", netsh: fakeNetsh{rules: []string{unnamedBlock}}, allow: true, clearBlocks: true, want: FirewallFailed},
		{name: "allow whitespace name block fails without elevation", netsh: fakeNetsh{rules: []string{blankBlock}}, allow: true, clearBlocks: true, want: FirewallFailed},
		{name: "same selector as allow rule needs manual cleanup", netsh: fakeNetsh{rules: []string{localBlock, collidingAllow}}, allow: true, clearBlocks: true, want: FirewallFailed},
		{name: "reserved all name needs manual cleanup", netsh: fakeNetsh{rules: []string{wildcardBlock}}, allow: true, clearBlocks: true, want: FirewallFailed},
		{name: "remove deletes the rule", netsh: fakeNetsh{rules: []string{validRule}}, allow: false, want: FirewallDone, verbs: []string{"delete"}},
		{name: "remove without a rule asks nothing", netsh: fakeNetsh{}, allow: false, want: FirewallDone},
		{name: "declined prompt", netsh: fakeNetsh{err: ErrElevationDeclined}, allow: true, want: FirewallDeclined, verbs: []string{"add"}},
		{name: "netsh fails", netsh: fakeNetsh{code: 1}, allow: true, want: FirewallFailed, verbs: []string{"add"}},
		{name: "rule still missing after netsh", netsh: fakeNetsh{noMutate: true}, allow: true, want: FirewallFailed, verbs: []string{"add"}},
		{name: "cannot query rules", netsh: fakeNetsh{inspectErr: errors.New("registry denied")}, allow: true, want: FirewallFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := tc.netsh
			if got := n.firewall().Apply(tc.allow, tc.clearBlocks); got != tc.want {
				t.Errorf("Apply(%v, %v) = %d, want %d", tc.allow, tc.clearBlocks, got, tc.want)
			}
			if tc.wantRules > 0 && len(n.rules) != tc.wantRules {
				t.Errorf("remaining rules = %d (%q), want %d", len(n.rules), n.rules, tc.wantRules)
			}
			if tc.verbs == nil {
				if len(n.batches) != 0 {
					t.Fatalf("unexpected elevation batches: %q", n.batches)
				}
				return
			}
			if len(n.batches) != 1 {
				t.Fatalf("expected 1 elevation batch, got %d: %q", len(n.batches), n.batches)
			}
			var gotVerbs []string
			for _, cmd := range n.batches[0] {
				gotVerbs = append(gotVerbs, cmd[2])
			}
			if !slices.Equal(gotVerbs, tc.verbs) {
				t.Errorf("batch verbs = %v, want %v (%q)", gotVerbs, tc.verbs, n.batches[0])
			}
		})
	}
}
