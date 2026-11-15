package qoder

import (
	"fmt"
	"strings"

	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

const (
	qoderPermissionsKey      = "permissions"
	qoderTrustDirectoriesKey = "trustDirectories"
)

// Trust is the international edition's independent folder-trust capability.
var Trust = NewTrustPolicy("qoder", []string{"qoder", "qodercli"}, "QODER_CONFIG_DIR", ".qoder")

// NewTrustPolicy describes the shared trust format of Qoder editions. The
// version-suffixed aliases deliberately retain trust's permissive matching.
func NewTrustPolicy(name string, executables []string, variable, dotDir string) trustconfig.Policy {
	return trustconfig.Policy{
		Name: name,
		Match: func(base string) bool {
			for _, exe := range executables {
				if base == exe || strings.HasPrefix(base, exe+"-") {
					return true
				}
			}
			return false
		},
		Plan: func(c trustconfig.Context, dir string) (trustconfig.Plan, error) {
			path, err := c.ConfigPath(variable, dotDir, "settings.json")
			return trustconfig.Plan{Path: path, Edit: func(data []byte) ([]byte, bool, error) { return qoderTrusted(data, dir) }}, err
		},
	}
}

func qoderTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	root, err := trustconfig.ObjectOf(data)
	if err != nil {
		return nil, false, err
	}
	permissions, err := trustconfig.ObjectOf(root[qoderPermissionsKey])
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", qoderPermissionsKey, err)
	}
	changed, err = trustconfig.AppendPath(permissions, qoderTrustDirectoriesKey, dir)
	if err != nil || !changed {
		return nil, false, err
	}
	if root[qoderPermissionsKey], err = trustconfig.EncodeJSON(permissions); err != nil {
		return nil, false, err
	}
	updated, err = trustconfig.IndentJSON(root)
	return updated, err == nil, err
}
