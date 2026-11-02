//go:build windows

package winui

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// uninstallKey is UNINST_KEY in deploy/windows/webmux.nsi.
const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\WebMux`

// InstalledCopy reports whether exe runs from the directory the installer
// registered as InstallLocation; any other tray is portable.
func InstalledCopy(exe string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = k.Close() }()
	location, _, err := k.GetStringValue("InstallLocation")
	if err != nil {
		return false
	}
	location = strings.TrimRight(strings.Trim(strings.TrimSpace(location), `"`), `\`)
	return samePath(location, filepath.Dir(exe))
}

// LaunchInstaller starts the downloaded installer silently, detached from
// the tray, without waiting: the installer closes this tray.
func LaunchInstaller(path string) error {
	cmd := exec.Command(path, "/S")
	cmd.Dir = filepath.Dir(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
