package update

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Errors returned by package updates.
var (
	ErrUnsupportedInstall = errors.New("当前安装方式不支持自动更新")
	ErrNotRoot            = errors.New("需要 root 权限")
)

// Method is how the running binary was installed.
type Method string

// Install methods reported by `webmux update check`.
const (
	MethodDeb   Method = "deb"
	MethodRPM   Method = "rpm"
	MethodOther Method = "other"
)

// Runner runs a program with argv (never a shell) and returns its combined
// output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// DetectMethod reports which package manager owns path: dpkg, then rpm.
// Missing tools or an unowned path yield MethodOther.
func DetectMethod(ctx context.Context, run Runner, path string) Method {
	if out, err := run(ctx, "dpkg-query", "-S", "--", path); err == nil {
		// "webmux: /usr/bin/webmux"; diversions print other lines first.
		for _, line := range strings.Split(string(out), "\n") {
			if pkg, file, ok := strings.Cut(line, ": "); ok && strings.TrimSpace(file) == path && packageName(pkg) == "webmux" {
				return MethodDeb
			}
		}
	}
	if out, err := run(ctx, "rpm", "-qf", "--qf", "%{NAME}\n", "--", path); err == nil && strings.TrimSpace(string(out)) == "webmux" {
		return MethodRPM
	}
	return MethodOther
}

// packageName drops a dpkg architecture qualifier ("webmux:amd64").
func packageName(s string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(s), ":")
	return name
}

// PackageAsset names the release package for method, version and GOARCH.
func PackageAsset(m Method, version, goarch string) (string, error) {
	debArch, rpmArch := "", ""
	switch goarch {
	case "amd64":
		debArch, rpmArch = "amd64", "x86_64"
	case "arm64":
		debArch, rpmArch = "arm64", "aarch64"
	default:
		return "", fmt.Errorf("没有 %s 架构的安装包", goarch)
	}
	switch m {
	case MethodDeb:
		return "webmux_" + version + "-1_" + debArch + ".deb", nil
	case MethodRPM:
		return "webmux-" + version + "-1." + rpmArch + ".rpm", nil
	}
	return "", ErrUnsupportedInstall
}

// installCommand is the package manager argv for a downloaded package.
func installCommand(m Method, file string) (string, []string) {
	if m == MethodRPM {
		return "rpm", []string{"-U", "--", file}
	}
	return "dpkg", []string{"-i", "--", file}
}
