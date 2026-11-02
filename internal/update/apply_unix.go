//go:build unix

package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ErrBusy means another apply holds the update lock.
var ErrBusy = errors.New("已有更新在进行")

// ApplyOptions configures one `webmux update apply` run.
type ApplyOptions struct {
	Current  string // running version
	Exe      string // installed executable, e.g. /usr/bin/webmux
	GOARCH   string
	Euid     int
	LockPath string // e.g. /run/webmux-update.lock
	CacheDir string // download directory, removed of packages afterwards
	Client   Client
	Run      Runner
	Out      io.Writer // progress lines for the system journal
}

// Apply installs a newer signed release package through dpkg or rpm. It
// never touches any user's webmux.service; running servers keep the old
// process until their users restart them.
func Apply(ctx context.Context, o ApplyOptions) error {
	if !IsRelease() {
		return ErrNotRelease
	}
	if o.Euid != 0 {
		return ErrNotRoot
	}
	unlock, err := lock(o.LockPath)
	if err != nil {
		return err
	}
	defer unlock()
	method := DetectMethod(ctx, o.Run, o.Exe)
	// Unsupported installs and architectures fail before any network use.
	if _, err := PackageAsset(method, o.Current, o.GOARCH); err != nil {
		return err
	}
	if Held(ctx, o.Run, method) {
		logf(o.Out, "webmux 软件包已被锁定版本（apt-mark hold / dnf versionlock），跳过自动更新")
		return nil
	}
	logf(o.Out, "当前版本 %s（%s 安装）", o.Current, method)
	m, host, err := o.Client.Latest(ctx)
	if err != nil {
		return fmt.Errorf("检查更新失败：%w", err)
	}
	if !Newer(m.Version, o.Current) {
		logf(o.Out, "已是最新版本 %s（来源 %s）", o.Current, host)
		return nil
	}
	logf(o.Out, "发现新版本 %s（来源 %s）", m.Version, host)
	asset, err := PackageAsset(method, m.Version, o.GOARCH)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(o.CacheDir, 0o700); err != nil {
		return fmt.Errorf("缓存目录：%w", err)
	}
	file := filepath.Join(o.CacheDir, asset)
	defer func() { _ = os.Remove(file) }()
	host, err = o.Client.Fetch(ctx, m, asset, file, 0o644)
	if err != nil {
		return fmt.Errorf("下载 %s 失败：%w", asset, err)
	}
	logf(o.Out, "已下载并校验 %s（来源 %s）", asset, host)
	name, args := installCommand(method, file)
	out, err := o.Run(ctx, name, args...)
	if err != nil {
		return fmt.Errorf("%s 安装失败：%w\n%s", name, err, strings.TrimSpace(string(out)))
	}
	logf(o.Out, "已安装 %s；正在运行的 WebMux 在重启后使用新版本", m.Version)
	return nil
}

func logf(w io.Writer, format string, args ...any) {
	if w != nil {
		_, _ = fmt.Fprintf(w, format+"\n", args...)
	}
}

// lock takes a non-blocking exclusive flock so only one apply runs.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("更新锁：%w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("更新锁：%w", err)
	}
	return func() { _ = f.Close() }, nil
}
