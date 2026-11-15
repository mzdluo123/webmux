package qodercn

import "github.com/rainchan/webmux/internal/agent/qoder"

// Trust retains this edition's aliases and config location independently of ACP.
var Trust = qoder.NewTrustPolicy("qodercn", []string{"qodercn", "qoderclicn", "qoder-cn"}, "QODERCN_CONFIG_DIR", ".qoder-cn")
