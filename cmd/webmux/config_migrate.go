package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/rainchan/webmux/internal/statefile"
)

// migrateConfig renames the v0.16.0 plugins root table to features before startup.
// TOML re-encoding normalizes formatting; the original formatting and comments
// remain in the private, exclusively published .pre-features.bak sidecar.
func migrateConfig(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	readInfo := info
	if info.Mode()&os.ModeSymlink != 0 {
		if readInfo, err = os.Stat(path); err != nil {
			return err
		}
	}
	if !readInfo.Mode().IsRegular() {
		return fmt.Errorf("configuration %s must be a regular file", path)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if _, err := toml.Decode(string(original), &document); err != nil {
		return err
	}
	plugins, legacy := document["plugins"]
	if !legacy {
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("legacy configuration %s must be a regular file (not a symbolic link)", path)
	}
	if _, exists := document["features"]; exists {
		return errors.New("configuration contains both [plugins] and [features]; resolve the conflict before starting")
	}
	if _, ok := plugins.(map[string]any); !ok {
		return errors.New("configuration plugins must be a TOML table")
	}
	document["features"] = plugins
	delete(document, "plugins")
	var migrated bytes.Buffer
	if err := toml.NewEncoder(&migrated).Encode(document); err != nil {
		return err
	}
	backup := path + ".pre-features.bak"
	if err := statefile.Write(backup, original, statefile.Options{
		TempPattern: ".config-migrate-*",
		Exclusive:   true,
	}); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("back up configuration: %w", err)
		}
		info, statErr := os.Lstat(backup)
		if statErr != nil {
			return fmt.Errorf("inspect configuration backup: %w", statErr)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			return fmt.Errorf("configuration backup %s must be a private regular file", backup)
		}
		saved, readErr := os.ReadFile(backup)
		if readErr != nil {
			return fmt.Errorf("read configuration backup: %w", readErr)
		}
		if !bytes.Equal(saved, original) {
			return fmt.Errorf("configuration backup %s differs from the original; refusing to overwrite it", backup)
		}
	}
	if err := statefile.Write(path, migrated.Bytes(), statefile.Options{TempPattern: ".config-migrate-*"}); err != nil {
		return fmt.Errorf("publish migrated configuration: %w", err)
	}
	return nil
}
