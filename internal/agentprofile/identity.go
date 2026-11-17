package agentprofile

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Signals are the bounded identity fields a session collected from its
// terminal output and process table. They are agent-neutral; an adapter
// decides which of them identify its sessions.
type Signals struct {
	// Warp cli-agent event fields.
	WarpAgent         string
	WarpPluginVersion string
	WarpSessionID     string
	WarpCwd           string
	// ProgramApp is the OSC 7501 root record's app.
	ProgramApp string
	// ProgramChildID is the first segment of the latest OSC 7501 child
	// record id.
	ProgramChildID string
	// RootExecutable and RootCwd describe the PTY's root process.
	RootExecutable string
	RootCwd        string
}

// Identity is an adapter's interpretation of Signals.
type Identity struct {
	Agent          string
	Version        string
	AgentSessionID string
	Cwd            string
}

// BelowVerified reports whether version is older than the newest release
// measured for the profile. Unparseable versions report false.
func (p *Profile) BelowVerified(version string) bool {
	order, ok := CompareVersions(version, p.VerifiedVersion)
	return ok && order < 0
}

// CompareVersions orders dotted numeric versions, ignoring a leading "v"
// and any hyphen/plus SemVer suffix while ordering PEP 440 pre/dev releases
// below the bare numeric core and post releases above it. ok is false if
// either version is unparseable.
func CompareVersions(a, b string) (order int, ok bool) {
	va, okA := parseVersion(a)
	vb, okB := parseVersion(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range max(len(va.parts), len(vb.parts)) {
		var x, y int
		if i < len(va.parts) {
			x = va.parts[i]
		}
		if i < len(vb.parts) {
			y = vb.parts[i]
		}
		if x != y {
			if x < y {
				return -1, true
			}
			return 1, true
		}
	}
	if c := compareInts(va.pepStage, vb.pepStage); c != 0 {
		return c, true
	}
	if c := compareInts(va.pepNum, vb.pepNum); c != 0 {
		return c, true
	}
	if c := compareInts(va.subStage, vb.subStage); c != 0 {
		return c, true
	}
	return compareInts(va.subNum, vb.subNum), true
}

func compareInts(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

type parsedVersion struct {
	parts    []int
	pepStage int // -4 dev, -3 alpha, -2 beta, -1 rc, 0 final, +1 post
	pepNum   int
	subStage int // optional trailing .devN (-1) or .postN (+1)
	subNum   int
}

func parseVersion(v string) (parsedVersion, bool) {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		v = v[1:]
	}
	if cut := strings.IndexAny(v, "-+ "); cut >= 0 {
		v = v[:cut]
	}
	if v == "" {
		return parsedVersion{}, false
	}
	fields := strings.Split(v, ".")
	parts := make([]int, 0, len(fields))
	for i, field := range fields {
		if field == "" {
			return parsedVersion{}, false
		}
		digits := leadingASCIIDigits(field)
		if digits == len(field) {
			n, err := strconv.Atoi(field)
			if err != nil || n < 0 {
				return parsedVersion{}, false
			}
			parts = append(parts, n)
			continue
		}
		// Inline PEP 440 suffix attached to a numeric segment after the first
		// dot, such as "3.0rc1" or "3.0.0rc1".
		if digits > 0 && i >= 1 {
			n, err := strconv.Atoi(field[:digits])
			if err != nil || n < 0 {
				return parsedVersion{}, false
			}
			pv, ok := parsePEP440Suffix(field[digits:], fields[i+1:])
			if !ok {
				return parsedVersion{}, false
			}
			pv.parts = append(parts, n)
			return pv, true
		}
		// Dotted PEP 440 suffix after at least major.minor, such as
		// "2.5.2.post1" or "2.5.dev0".
		if digits == 0 && i >= 2 {
			pv, ok := parsePEP440Suffix(field, fields[i+1:])
			if !ok {
				return parsedVersion{}, false
			}
			pv.parts = parts
			return pv, true
		}
		return parsedVersion{}, false
	}
	return parsedVersion{parts: parts}, len(parts) > 0
}

func leadingASCIIDigits(s string) int {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return i
		}
	}
	return len(s)
}

func parsePEP440Suffix(first string, rest []string) (parsedVersion, bool) {
	stage, num, ok := parsePEP440Segment(first)
	if !ok || len(rest) > 1 {
		return parsedVersion{}, false
	}
	pv := parsedVersion{pepStage: stage, pepNum: num}
	if len(rest) == 1 {
		if stage == -4 {
			return parsedVersion{}, false
		}
		subStage, subNum, subOK := parsePEP440Segment(rest[0])
		if !subOK || (subStage != 1 && subStage != -4) || (stage == 1 && subStage == 1) {
			return parsedVersion{}, false
		}
		if subStage == -4 {
			subStage = -1
		}
		pv.subStage = subStage
		pv.subNum = subNum
	}
	return pv, true
}

func parsePEP440Segment(seg string) (stage, num int, ok bool) {
	alpha := leadingASCIILetters(seg)
	if alpha == 0 {
		return 0, 0, false
	}
	tag := strings.ToLower(seg[:alpha])
	rest := seg[alpha:]
	if leadingASCIIDigits(rest) != len(rest) {
		return 0, 0, false
	}
	switch tag {
	case "dev":
		stage = -4
	case "alpha":
		stage = -3
	case "a":
		if rest == "" {
			return 0, 0, false
		}
		stage = -3
	case "beta":
		stage = -2
	case "b":
		if rest == "" {
			return 0, 0, false
		}
		stage = -2
	case "rc", "pre", "preview":
		stage = -1
	case "c":
		if rest == "" {
			return 0, 0, false
		}
		stage = -1
	case "post", "rev":
		stage = 1
	case "r":
		if rest == "" {
			return 0, 0, false
		}
		stage = 1
	default:
		return 0, 0, false
	}
	if rest != "" {
		n, err := strconv.Atoi(rest)
		if err != nil || n < 0 {
			return 0, 0, false
		}
		num = n
	}
	return stage, num, true
}

func leadingASCIILetters(s string) int {
	for i := range len(s) {
		b := s[i]
		if (b < 'a' || b > 'z') && (b < 'A' || b > 'Z') {
			return i
		}
	}
	return len(s)
}

// LockLayout is where an agent holds a writer lock for each session it has
// open: <home>/<Dir>/<id><Suffix>, home from environment variable HomeEnv
// or else HomeDefault under the user's home directory.
type LockLayout struct {
	HomeEnv, HomeDefault, Dir, Suffix string
}

// Sessions returns the session IDs whose writer locks are among paths, in
// paths order (see LockTracker). The lock directory is compared with
// symbolic links resolved, as the process table reports open files.
func (l LockLayout) Sessions(paths []string, getenv func(string) string, userHome string) []string {
	home := getenv(l.HomeEnv)
	if home == "" {
		if userHome == "" {
			return nil
		}
		home = filepath.Join(userHome, l.HomeDefault)
	}
	dir := filepath.Join(home, l.Dir)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	var ids []string
	for _, path := range paths {
		if filepath.Dir(path) != dir {
			continue
		}
		id, ok := strings.CutSuffix(filepath.Base(path), l.Suffix)
		if ok && sessionLockID(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// sessionLockID accepts UUID-shaped names: hex digits and dashes, 36 bytes.
// The lock directory also holds other locks (.coordination.lock).
func sessionLockID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c == '-', c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
