package codex

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

// Trust describes folder trust without requiring runtime/ACP support.
var Trust = trustconfig.Policy{
	Name:  "codex",
	Match: func(base string) bool { return base == "codex" },
	Plan:  planTrust,
}

func planTrust(c trustconfig.Context, dir string) (trustconfig.Plan, error) {
	path, err := c.ConfigPath("CODEX_HOME", ".codex", "config.toml")
	return trustconfig.Plan{Path: path, Create: true, RequireParent: true, Edit: func(data []byte) ([]byte, bool, error) { return codexTrusted(data, dir) }}, err
}

// codexTrusted appends a project table instead of re-encoding the file, which
// keeps the user's comments and ordering. An existing table for dir, trusted
// or explicitly untrusted, is the user's decision and is left as it is.
func codexTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	var doc struct {
		Projects map[string]map[string]any `toml:"projects"`
	}
	if _, err = toml.Decode(string(data), &doc); err != nil {
		return nil, false, err
	}
	if _, decided := doc.Projects[dir]; decided {
		return nil, false, nil
	}
	key, err := tomlKey(dir)
	if err != nil {
		return nil, false, err
	}
	var out bytes.Buffer
	out.Write(data)
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		out.WriteByte('\n')
	}
	if len(data) > 0 {
		out.WriteByte('\n')
	}
	fmt.Fprintf(&out, "[projects.%s]\ntrust_level = \"trusted\"\n", key)
	return out.Bytes(), true, nil
}

// tomlKey quotes s as a TOML basic-string key.
func tomlKey(s string) (string, error) {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			return "", fmt.Errorf("directory %q has a control character", s)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String(), nil
}
