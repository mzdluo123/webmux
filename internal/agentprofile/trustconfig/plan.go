// Package trustconfig describes agent-owned trust edits without performing I/O.
package trustconfig

import "path/filepath"

// Context resolves the launch environment without reading configuration files.
type Context struct {
	Home      func() (string, error)
	Env       func(string) string
	LookupEnv func(string) (string, bool)
}

// ConfigPath selects an agent override directory or its home-relative default.
func (c Context) ConfigPath(variable, dotDir, name string) (string, error) {
	if dir := c.Env(variable); dir != "" {
		return filepath.Join(dir, name), nil
	}
	home, err := c.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dotDir, name), nil
}

// Plan describes effects executed under the trust service's single lock.
// Environment overrides are process-scoped and never persisted.
type Plan struct {
	Path          string
	Create        bool
	CreateParents bool
	RequireParent bool
	Edit          func([]byte) ([]byte, bool, error)
	Environment   map[string]string
}

// Policy is independent of runtime/ACP support. Match receives each argv word's
// basename, preserving the trust service's deliberately permissive scanning.
type Policy struct {
	Name  string
	Match func(string) bool
	Plan  func(Context, string) (Plan, error)
}
