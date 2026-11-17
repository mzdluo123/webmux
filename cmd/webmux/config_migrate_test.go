package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
)

func TestLoadConfigMigratesLegacyPlugins(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, tt := range []struct {
			name       string
			body       string
			nilEnabled bool
		}{
			{name: "table", body: "[plugins]\nenabled = ['code']\n"},
			{name: "dotted", body: "plugins.enabled = []\n"},
			{name: "quoted table", body: "['plugins']\n'enabled' = []\n"},
			{name: "quoted dotted", body: "\"plugins\".\"enabled\" = []\n"},
			{name: "inline", body: "plugins = { enabled = [] }\n"},
			{name: "empty table", body: "[plugins]\n", nilEnabled: true},
		} {
			name := "default/" + tt.name
			if explicit {
				name = "explicit/" + tt.name
			}
			t.Run(name, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				t.Setenv("XDG_STATE_HOME", "")
				path := defaultConfigPath(home)
				var args []string
				if explicit {
					path = filepath.Join(home, "custom.toml")
					args = []string{"--config", path}
				}
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				original := []byte("# Keep this original comment in the backup.\nlisten = '127.0.0.1:9191'\ntoken = 'private-token-#-[plugins]-value'\nallowed_origins = ['https://example.test']\n" + tt.body)
				if err := os.WriteFile(path, original, 0640); err != nil {
					t.Fatal(err)
				}
				run, err := loadConfig(args)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(run.closeSettings)
				if run.ConfigCreated || run.ConfigExplicit != explicit || run.TokenGenerated || run.Token != "private-token-#-[plugins]-value" || run.Listen != "127.0.0.1:9191" {
					t.Fatalf("startup configuration changed: %#v", run)
				}
				if (run.Features.Enabled == nil) != tt.nilEnabled {
					t.Fatalf("enabled nil = %v, want %v", run.Features.Enabled == nil, tt.nilEnabled)
				}
				var before, after map[string]any
				if _, err := toml.Decode(string(original), &before); err != nil {
					t.Fatal(err)
				}
				migrated, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := toml.Decode(string(migrated), &after); err != nil {
					t.Fatal(err)
				}
				before["features"] = before["plugins"]
				delete(before, "plugins")
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("migration changed configuration values:\nwant %#v\ngot %#v", before, after)
				}
				backup := path + ".pre-features.bak"
				saved, err := os.ReadFile(backup)
				if err != nil || !bytes.Equal(saved, original) {
					t.Fatalf("original backup changed: %q, %v", saved, err)
				}
				for _, file := range []string{path, backup} {
					info, err := os.Stat(file)
					if err != nil || info.Mode().Perm() != 0600 {
						t.Fatalf("private file %s: %v, %v", file, info, err)
					}
					if err := migrateConfig(path); err != nil {
						t.Fatal(err)
					}
					again, err := os.Stat(file)
					if err != nil || !os.SameFile(info, again) {
						t.Fatalf("idempotent migration rewrote %s: %v", file, err)
					}
				}
			})
		}
	}
}

func TestMigrateConfigLeavesCurrentConfigurationUntouched(t *testing.T) {
	for _, content := range []string{
		"# unchanged\ntoken = 'plugins'\n",
		"# unchanged\n[features]\nenabled = []\n",
		"[other.plugins]\nenabled = []\n",
	} {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(content), 0640); err != nil {
			t.Fatal(err)
		}
		before, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := migrateConfig(path); err != nil {
			t.Fatal(err)
		}
		after, err := os.Stat(path)
		if err != nil || !os.SameFile(before, after) || after.Mode().Perm() != 0640 {
			t.Fatalf("current configuration was republished: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Fatalf("current configuration changed: %q, %v", got, err)
		}
		if _, err := os.Lstat(path + ".pre-features.bak"); !os.IsNotExist(err) {
			t.Fatalf("unexpected backup: %v", err)
		}
	}
}

func TestMigrateConfigRejectsInvalidInputWithoutChanges(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "conflicting tables", body: "[plugins]\nenabled = []\n[features]\nenabled = ['code']\n"},
		{name: "conflicting dotted and inline", body: "plugins.enabled = []\nfeatures = {enabled = []}\n"},
		{name: "conflicting empty tables", body: "[plugins]\n[features]\n"},
		{name: "syntax error", body: "[plugins]\nenabled = [\n"},
		{name: "duplicate key", body: "plugins.enabled = []\nplugins.enabled = []\n"},
		{name: "scalar", body: "plugins = false\n"},
		{name: "array of tables", body: "[[plugins]]\nenabled = []\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(tt.body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadConfig([]string{"--config", path}); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != tt.body {
				t.Fatalf("invalid input was changed: %q, %v", got, err)
			}
			if _, err := os.Lstat(path + ".pre-features.bak"); !os.IsNotExist(err) {
				t.Fatalf("invalid input created a backup: %v", err)
			}
		})
	}
}

func TestMigrateConfigBackupIsExclusiveAndRetryable(t *testing.T) {
	const original = "# original\n[plugins]\nenabled = []\n"
	for _, saved := range []string{original, "unrelated backup"} {
		t.Run(saved, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			backup := path + ".pre-features.bak"
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(backup, []byte(saved), 0600); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(backup)
			if err != nil {
				t.Fatal(err)
			}
			err = migrateConfig(path)
			if (err == nil) != (saved == original) {
				t.Fatalf("migration with existing backup: %v", err)
			}
			after, err := os.Stat(backup)
			if err != nil || !os.SameFile(info, after) {
				t.Fatalf("existing backup was replaced: %v", err)
			}
			got, err := os.ReadFile(backup)
			if err != nil || string(got) != saved {
				t.Fatalf("existing backup changed: %q, %v", got, err)
			}
			got, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if saved != original && string(got) != original {
				t.Fatal("backup conflict changed the original")
			}
			if saved == original {
				c, err := config.Load(path, true)
				if err != nil || c.Features.Enabled == nil || len(c.Features.Enabled) != 0 {
					t.Fatalf("retry did not preserve explicit empty features: %#v, %v", c.Features, err)
				}
			}
		})
	}
}

func TestMigrateConfigRejectsNonRegularPaths(t *testing.T) {
	for _, target := range []string{"config", "backup"} {
		for _, kind := range []string{"directory", "symlink"} {
			t.Run(target+"/"+kind, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "config.toml")
				const original = "[plugins]\nenabled = []\n"
				protected := filepath.Join(dir, "original.toml")
				if err := os.WriteFile(protected, []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
				badPath := path
				if target == "backup" {
					if err := os.WriteFile(path, []byte(original), 0600); err != nil {
						t.Fatal(err)
					}
					badPath += ".pre-features.bak"
				}
				if kind == "directory" {
					if err := os.Mkdir(badPath, 0700); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Symlink(protected, badPath); err != nil {
					t.Fatal(err)
				}
				if err := migrateConfig(path); err == nil {
					t.Fatal("non-regular path accepted")
				}
				info, err := os.Lstat(badPath)
				if err != nil || (kind == "directory" && !info.IsDir()) || (kind == "symlink" && info.Mode()&os.ModeSymlink == 0) {
					t.Fatalf("non-regular path changed: %v, %v", info, err)
				}
				for _, file := range []string{protected, path} {
					if file == path && target == "config" && kind == "directory" {
						continue
					}
					got, err := os.ReadFile(file)
					if err != nil || string(got) != original {
						t.Fatalf("original changed: %q, %v", got, err)
					}
				}
			})
		}
	}
}

func TestMigrateConfigWriteFailurePreservesOriginal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directories do not stop root")
	}
	for _, existingBackup := range []bool{false, true} {
		t.Run(map[bool]string{false: "without backup", true: "with existing backup"}[existingBackup], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			const original = "# unchanged on failure\n[plugins]\nenabled = []\n"
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			if existingBackup {
				if err := os.WriteFile(path+".pre-features.bak", []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(dir, 0500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
			if err := migrateConfig(path); err == nil {
				t.Fatal("write failure was ignored")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != original {
				t.Fatalf("write failure changed original: %q, %v", got, err)
			}
		})
	}
}

func TestReadOnlyConfigCommandsDoNotMigrateLegacyPlugins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	path := defaultConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	const original = "# original\n[plugins]\nenabled = []\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"show", "validate"} {
		for _, explicit := range []bool{false, true} {
			args := []string{"config", command}
			if explicit {
				args = append(args, "--config", path)
			}
			var stdout, stderr bytes.Buffer
			if code := dispatch(args, &stdout, &stderr); code != contract.ExitError {
				t.Fatalf("%v: exit %d, want diagnostic failure", args, code)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != original {
				t.Fatalf("%v changed original: %q, %v", args, got, err)
			}
			if _, err := os.Lstat(path + ".pre-features.bak"); !os.IsNotExist(err) {
				t.Fatalf("%v created backup: %v", args, err)
			}
			if command == "validate" && !strings.Contains(stderr.String(), "[features]") {
				t.Fatalf("legacy diagnostic lost: %s", &stderr)
			}
		}
	}
}

func TestLoadConfigKeepsCurrentConfigSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "current.toml")
	path := filepath.Join(dir, "config.toml")
	const content = "# keep current format\n[features]\nenabled = []\n"
	if err := os.WriteFile(target, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	run, err := loadConfig([]string{"--config", path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(run.closeSettings)
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("current config symlink changed: %v, %v", info, err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != content {
		t.Fatalf("current target changed: %q, %v", got, err)
	}
	if _, err := os.Lstat(path + ".pre-features.bak"); !os.IsNotExist(err) {
		t.Fatalf("current config created a backup: %v", err)
	}
}
