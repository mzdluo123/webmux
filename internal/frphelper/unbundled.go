//go:build !webmux_frpc || (!linux && !darwin) || (!amd64 && !arm64)

package frphelper

var bundledBinary, bundledLicense []byte
