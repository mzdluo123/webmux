//go:build webmux_frpc && (linux || darwin) && (amd64 || arm64)

package frphelper

import _ "embed"

// The release build supplies these two files through its platform overlay.

//go:embed payload/frpc
var bundledBinary []byte

//go:embed payload/LICENSE
var bundledLicense []byte
