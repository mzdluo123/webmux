package relaysetting

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/statefile"
)

func storageTestSettings() Settings {
	seed := make([]byte, ed25519.SeedSize)
	pending := make([]byte, ed25519.SeedSize)
	pending[0] = 1
	return Settings{
		Config:      config.RelayConfig{Enabled: true, Transport: "wss", ControlEndpoint: "https://control.example.com", ControlAudience: "webmux-control"},
		Seed:        base64.RawURLEncoding.EncodeToString(seed),
		PendingSeed: base64.RawURLEncoding.EncodeToString(pending),
		Binding:     &protocol.Binding{HostID: "0123456789abcdef0123456789abcdef", OwnerID: "12345", BindingGeneration: 7, PublicURL: "https://h-0123456789abcdef0123456789abcdef.hosts.example.com"},
		OwnerLogin:  "owner-name",
		Confirmed:   true,
	}
}

func storageTestDocument(t *testing.T, settings Settings) []byte {
	t.Helper()
	data, err := json.Marshal(storeSpec.Encode(settings))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func legacyStorageTestDocument(t *testing.T, settings Settings) []byte {
	t.Helper()
	f := storedFileV1{V: 1, Enabled: &settings.Config.Enabled, Confirmed: &settings.Confirmed, ControlEndpoint: settings.Config.ControlEndpoint, ControlAudience: settings.Config.ControlAudience, Seed: settings.Seed, PendingSeed: settings.PendingSeed, Binding: settings.Binding, OwnerLogin: settings.OwnerLogin}
	if f.ControlEndpoint != "" {
		f.InitialTrust = protocol.ServiceTrust{Algorithm: protocol.ServiceAlgorithm, Pins: []string{storageTestSettings().Seed}}
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRelayV1MigrationPreservesBindingAndPairingState(t *testing.T) {
	for _, state := range []string{"enabled", "disabled", "pending", "unconfigured"} {
		t.Run(state, func(t *testing.T) {
			want := storageTestSettings()
			switch state {
			case "disabled":
				want.Config.Enabled = false
			case "pending":
				want.Config.Enabled = false
				want.Seed, want.Binding, want.OwnerLogin, want.Confirmed = "", nil, "", false
			case "unconfigured":
				want = Settings{Config: config.RelayConfig{Transport: "wss"}}
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "relay-settings.json")
			legacy := legacyStorageTestDocument(t, want)
			if err := os.WriteFile(path, legacy, 0600); err != nil {
				t.Fatal(err)
			}
			got, web, err := Inspect(dir, want.Config)
			if err != nil || !web || !reflect.DeepEqual(got, want) {
				t.Fatalf("legacy inspection lost pairing state: web=%v err=%v", web, err)
			}
			s := Open(dir, want.Config)
			t.Cleanup(s.Close)
			if err := s.Unavailable(); err != nil {
				t.Fatal(err)
			}
			got, web = s.Effective()
			if !web || !reflect.DeepEqual(got, want) {
				t.Fatal("legacy load lost identity, consent, Origin or enabled intent")
			}
			if err := s.Save(got); err != nil {
				t.Fatal(err)
			}
			s.Close()
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]json.RawMessage
			if err := json.Unmarshal(data, &document); err != nil {
				t.Fatal(err)
			}
			if string(document["v"]) != "2" || document["initialTrust"] != nil {
				t.Fatal("migrated save retained the legacy control trust schema")
			}
			reopened := Open(dir, want.Config)
			defer reopened.Close()
			if err := reopened.Unavailable(); err != nil {
				t.Fatal(err)
			}
			if got, web := reopened.Effective(); !web || !reflect.DeepEqual(got, want) {
				t.Fatal("migration did not preserve state after restart")
			}
		})
	}
}

func TestRelayRestoresIdentityAndEnabledIntent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	want := storageTestSettings()
	s := Open(dir, want.Config)
	if err := s.Unavailable(); err != nil {
		t.Fatal(err)
	}
	if got, web := s.Effective(); got.Config.Enabled || web || got.Seed != "" || got.Binding != nil {
		t.Fatal("TOML enabled an unpaired host")
	}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if runtime.GOOS != "windows" {
		for _, item := range []struct {
			path string
			mode os.FileMode
		}{{dir, 0700}, {filepath.Join(dir, "relay-settings.json"), 0600}, {filepath.Join(dir, "relay-settings.json.lock"), 0600}} {
			info, err := os.Stat(item.path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != item.mode {
				t.Fatalf("identity storage mode = %o, want %o", info.Mode().Perm(), item.mode)
			}
		}
	}
	inspected, web, err := Inspect(dir, want.Config)
	if err != nil || !web || !reflect.DeepEqual(inspected, want) {
		t.Fatalf("identity snapshot did not restore: web=%v err=%v", web, err)
	}
	s = Open(dir, want.Config)
	defer s.Close()
	if err := s.Unavailable(); err != nil {
		t.Fatal(err)
	}
	if got, web := s.Effective(); !web || !reflect.DeepEqual(got, want) {
		t.Fatal("restart lost saved identity or enabled intent")
	}
	reset, err := s.Reset()
	if err != nil {
		t.Fatal(err)
	}
	if reset.Config.Enabled || reset.Seed != "" || reset.PendingSeed != "" || reset.Binding != nil || reset.Confirmed || reset.OwnerLogin != "" {
		t.Fatal("reset retained identity or enabled TOML exposure")
	}
	if _, err := os.Stat(filepath.Join(dir, "relay-settings.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reset left identity file: %v", err)
	}
}

func TestRelayDamagedStorageFailsClosedWithoutChangingEvidence(t *testing.T) {
	valid := storageTestSettings()
	good := storageTestDocument(t, valid)
	cases := map[string][]byte{
		"truncated":       good[:len(good)-1],
		"empty":           {},
		"trailing":        append(append([]byte(nil), good...), []byte(` {}`)...),
		"unknown field":   append([]byte(`{"unexpected":true,`), good[1:]...),
		"duplicate field": append([]byte(`{"enabled":false,`), good[1:]...),
	}
	legacy := legacyStorageTestDocument(t, valid)
	cases["legacy unknown field"] = append([]byte(`{"unexpected":true,`), legacy[1:]...)
	cases["legacy duplicate field"] = append([]byte(`{"enabled":false,`), legacy[1:]...)
	cases["legacy invalid pin"] = bytes.Replace(legacy, []byte(valid.Seed), []byte("bad-pin"), 1)
	cases["v2 legacy trust field"] = append([]byte(`{"initialTrust":{},`), good[1:]...)
	for name, mutate := range map[string]func(*storedFile){
		"wrong version":         func(f *storedFile) { f.V = 3 },
		"missing enabled":       func(f *storedFile) { f.Enabled = nil },
		"missing consent":       func(f *storedFile) { f.Confirmed = nil },
		"malformed key":         func(f *storedFile) { f.Seed = "bad-seed" },
		"noncanonical key":      func(f *storedFile) { f.Seed = strings.Repeat("A", 42) + "B" },
		"malformed pending key": func(f *storedFile) { f.PendingSeed = "bad-seed" },
		"binding without key":   func(f *storedFile) { f.Seed = "" },
		"invalid binding":       func(f *storedFile) { f.Binding.OwnerID = "not-numeric" },
		"unconfirmed exposure":  func(f *storedFile) { f.Confirmed = new(bool) },
		"unpaired exposure":     func(f *storedFile) { f.Binding = nil },
		"missing bootstrap": func(f *storedFile) {
			f.ControlEndpoint = ""
			f.ControlAudience = ""
			f.Enabled = new(bool)
		},
	} {
		f := storeSpec.Encode(storageTestSettings())
		mutate(&f)
		data, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		cases[name] = data
	}
	for name, damaged := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "relay-settings.json")
			if err := os.WriteFile(path, damaged, 0600); err != nil {
				t.Fatal(err)
			}
			s := Open(dir, valid.Config)
			defer s.Close()
			if got, web := s.Effective(); got.Config.Enabled || web || got.Binding != nil || got.Seed != "" {
				t.Fatal("damaged identity enabled exposure or restored partial identity")
			}
			if !errors.Is(s.Unavailable(), ErrUnavailable) {
				t.Fatal("damaged identity storage not reported unavailable")
			}
			if !errors.Is(s.Save(valid), ErrUnavailable) {
				t.Fatal("damaged evidence could be overwritten")
			}
			if got, web, err := Inspect(dir, valid.Config); err == nil || web || got.Config.Enabled {
				t.Fatal("inspection accepted damaged identity or enabled TOML fallback")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, damaged) {
				t.Fatal("damaged identity file changed")
			}
		})
	}
}

func TestRelayRejectsInvalidUpdatesWithoutReplacingIdentity(t *testing.T) {
	valid := storageTestSettings()
	dir := t.TempDir()
	s := Open(dir, valid.Config)
	defer s.Close()
	if err := s.Save(valid); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "relay-settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Settings)
		want   error
	}{
		{"unpaired enable", func(s *Settings) { s.Binding = nil }, ErrEnableBlocked},
		{"unconfirmed enable", func(s *Settings) { s.Confirmed = false }, ErrEnableBlocked},
		{"missing key", func(s *Settings) { s.Seed = "" }, ErrInvalid},
		{"bad pending key", func(s *Settings) { s.PendingSeed = "bad" }, ErrInvalid},
		{"unbound key without authority", func(s *Settings) { s.Config = config.RelayConfig{}; s.Binding = nil }, ErrInvalid},
		{"changed endpoint", func(s *Settings) { s.Config.ControlEndpoint = "https://other.example.com" }, ErrInvalid},
		{"changed audience", func(s *Settings) { s.Config.ControlAudience = "other-control" }, ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := storageTestSettings()
			tc.mutate(&next)
			if err := s.Save(next); !errors.Is(err, tc.want) {
				t.Fatalf("rejected update error = %v, want %v", err, tc.want)
			}
			if got, web := s.Effective(); !web || !reflect.DeepEqual(got, valid) {
				t.Fatal("rejected update changed effective identity")
			}
			after, err := os.ReadFile(filepath.Join(dir, "relay-settings.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("rejected update changed persisted identity")
			}
			if err := s.Unavailable(); err != nil {
				t.Fatalf("validation poisoned healthy storage: %v", err)
			}
		})
	}
}

func TestRelayBootstrapChangesDoNotRestoreOldCredentials(t *testing.T) {
	valid := storageTestSettings()
	for _, tc := range []struct {
		name   string
		mutate func(*config.RelayConfig)
	}{
		{"endpoint", func(c *config.RelayConfig) { c.ControlEndpoint = "https://other.example.com" }},
		{"audience", func(c *config.RelayConfig) { c.ControlAudience = "other-control" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for version, document := range map[string][]byte{"v1": legacyStorageTestDocument(t, valid), "v2": storageTestDocument(t, valid)} {
				t.Run(version, func(t *testing.T) {
					dir := t.TempDir()
					if err := os.WriteFile(filepath.Join(dir, "relay-settings.json"), document, 0600); err != nil {
						t.Fatal(err)
					}
					defaults := storageTestSettings().Config
					tc.mutate(&defaults)
					s := Open(dir, defaults)
					defer s.Close()
					if got, _ := s.Effective(); got.Config.Enabled || got.Seed != "" || got.PendingSeed != "" || got.Binding != nil {
						t.Fatal("bootstrap change restored credentials for another authority")
					}
					if !errors.Is(s.Unavailable(), ErrUnavailable) {
						t.Fatal("bootstrap change did not fail closed")
					}
					if _, _, err := Inspect(dir, defaults); err == nil {
						t.Fatal("inspection restored credentials across bootstrap change")
					}
				})
			}
		})
	}
}

func TestRelayLockConflictCannotRestoreOrOverwriteIdentity(t *testing.T) {
	valid := storageTestSettings()
	dir := t.TempDir()
	first := Open(dir, valid.Config)
	defer first.Close()
	if err := first.Save(valid); err != nil {
		t.Fatal(err)
	}
	second := Open(dir, valid.Config)
	defer second.Close()
	if !errors.Is(second.Unavailable(), ErrUnavailable) || !statefile.IsKind(second.Unavailable(), statefile.Locked) {
		t.Fatal("second identity writer did not report lock conflict")
	}
	if got, _ := second.Effective(); got.Config.Enabled || got.Binding != nil || got.Seed != "" {
		t.Fatal("lock conflict restored active identity")
	}
	if !errors.Is(second.Save(valid), ErrUnavailable) {
		t.Fatal("lock-conflicted writer saved identity")
	}
	if got, _ := first.Effective(); !reflect.DeepEqual(got, valid) {
		t.Fatal("lock conflict changed active owner identity")
	}
}

func TestRelayPrivateKeyCanonicalSeedAndIndependentVector(t *testing.T) {
	// RFC 8032 test 1 fixes both the derived public key and empty-message signature.
	seed, err := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
	if err != nil {
		t.Fatal(err)
	}
	key, err := PrivateKey(base64.RawURLEncoding.EncodeToString(seed))
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(key.Public().(ed25519.PublicKey)) != "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a" {
		t.Fatal("seed derived the wrong host identity")
	}
	if hex.EncodeToString(ed25519.Sign(key, nil)) != "e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b" {
		t.Fatal("host identity failed independent signature vector")
	}
	for _, seed := range []string{"", "bad", strings.Repeat("A", 42), strings.Repeat("A", 44), strings.Repeat("A", 42) + "B", strings.Repeat("A", 42) + "=", strings.Repeat("A", 42) + "+", strings.Repeat("A", 4096)} {
		if key, err := PrivateKey(seed); !errors.Is(err, ErrInvalid) || key != nil {
			t.Fatal("malformed seed returned usable identity")
		}
	}
}

func TestRelayDisableRetainsIdentityForReconnectionAndDeletion(t *testing.T) {
	dir := t.TempDir()
	want := storageTestSettings()
	s := Open(dir, want.Config)
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	want.Config.Enabled = false
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = Open(dir, storageTestSettings().Config)
	defer s.Close()
	if err := s.Unavailable(); err != nil {
		t.Fatal(err)
	}
	if got, web := s.Effective(); !web || !reflect.DeepEqual(got, want) {
		t.Fatal("disabled restart lost the binding, active key, pending key or consent")
	}
}

func TestRelayUnpairedIdentityRequiresControlAuthority(t *testing.T) {
	for _, pending := range []bool{false, true} {
		s := Open(t.TempDir(), config.RelayConfig{})
		next := Settings{}
		if pending {
			next.PendingSeed = storageTestSettings().PendingSeed
		} else {
			next.Seed = storageTestSettings().Seed
		}
		if err := s.Save(next); !errors.Is(err, ErrInvalid) {
			s.Close()
			t.Fatalf("identity without trusted authority = %v", err)
		}
		if err := s.Unavailable(); err != nil {
			s.Close()
			t.Fatal(err)
		}
		s.Close()
	}
}

func TestRelaySaveCopiesMutableIdentity(t *testing.T) {
	want := storageTestSettings()
	s := Open(t.TempDir(), want.Config)
	defer s.Close()
	next := storageTestSettings()
	if err := s.Save(next); err != nil {
		t.Fatal(err)
	}
	next.Binding.OwnerID = "54321"
	if got, _ := s.Effective(); !reflect.DeepEqual(got, want) {
		t.Fatal("caller mutation changed the accepted identity or authority")
	}
}

func TestRelayUnconfiguredStorageAdoptsDefaultsWithoutRedirectingIdentity(t *testing.T) {
	for _, withIdentity := range []bool{false, true} {
		settings := Settings{Config: config.RelayConfig{Transport: "wss"}}
		if withIdentity {
			settings.Seed = storageTestSettings().Seed
		}
		for version, document := range map[string][]byte{"v1": legacyStorageTestDocument(t, settings), "v2": storageTestDocument(t, settings)} {
			t.Run(fmt.Sprintf("%s/identity=%t", version, withIdentity), func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "relay-settings.json")
				if err := os.WriteFile(path, document, 0600); err != nil {
					t.Fatal(err)
				}
				s := Open(dir, config.DefaultRelayConfig())
				defer s.Close()
				got, _ := s.Effective()
				if withIdentity {
					if !errors.Is(s.Unavailable(), ErrUnavailable) || got.Seed != "" || got.Config.Enabled {
						t.Fatal("identity without an authority was redirected to the defaults")
					}
				} else if s.Unavailable() != nil || got.Config != config.DefaultRelayConfig() {
					t.Fatalf("unpaired legacy settings prevented default bootstrap: %+v, %v", got, s.Unavailable())
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(after, document) {
					t.Fatal("loading defaults rewrote persisted evidence")
				}
			})
		}
	}
}
