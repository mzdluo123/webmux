package agentprofile

import "strings"

// UpstreamKind identifies the public registry or release API that publishes an
// agent's or connector's latest stable version.
type UpstreamKind string

const (
	// UpstreamNone means no public package registry is queried automatically.
	UpstreamNone UpstreamKind = ""
	// UpstreamNPM queries an npm registry /<package>/latest endpoint.
	UpstreamNPM UpstreamKind = "npm"
	// UpstreamPyPI queries PyPI's /pypi/<package>/json endpoint.
	UpstreamPyPI UpstreamKind = "pypi"
	// UpstreamGitHubRelease queries GitHub's /repos/<owner>/<repo>/releases/latest endpoint.
	UpstreamGitHubRelease UpstreamKind = "github_release"
)

// UpstreamSource describes where to check the latest published release version.
type UpstreamSource struct {
	Kind    UpstreamKind
	Package string
}

// NPMUpstream constructs an npm package upstream source.
func NPMUpstream(pkg string) UpstreamSource {
	return UpstreamSource{Kind: UpstreamNPM, Package: pkg}
}

// PyPIUpstream constructs a PyPI package upstream source.
func PyPIUpstream(pkg string) UpstreamSource {
	return UpstreamSource{Kind: UpstreamPyPI, Package: pkg}
}

// GitHubReleaseUpstream constructs a GitHub repository latest-release source.
func GitHubReleaseUpstream(repo string) UpstreamSource {
	return UpstreamSource{Kind: UpstreamGitHubRelease, Package: repo}
}

// Distribution describes how users install, upgrade, and check releases for an
// agent CLI or connector. WebMux surfaces these commands as copyable guidance
// and never runs them silently in the background.
type Distribution struct {
	InstallCommand string
	UpgradeCommand string
	DocsURL        string
	Upstream       UpstreamSource
}

// ExtractVersion returns the first dotted numeric version token found in the
// initial lines of a --version output (for example "2.1.316 (Claude Code)",
// "codex-cli 0.165.0", "omp/18.20.1", or "kimi, version 2.5.2"), stripped of
// any leading "v"/"V". It returns "" when no dotted version is present.
func ExtractVersion(out string) string {
	const maxLines = 8
	lines := strings.Split(out, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for _, line := range lines {
		for _, field := range strings.Fields(line) {
			if v := normalizeVersionCandidate(field); v != "" {
				return v
			}
			// Handle slash/equals/colon-prefixed tokens such as "omp/18.20.1"
			// or "version=2.1.4".
			for _, sep := range []string{"/", "=", ":"} {
				if idx := strings.LastIndex(field, sep); idx >= 0 {
					if v := normalizeVersionCandidate(field[idx+1:]); v != "" {
						return v
					}
				}
			}
		}
	}
	return ""
}

func normalizeVersionCandidate(token string) string {
	token = strings.Trim(token, "()[]{}<>,;:\"'`")
	if strings.HasPrefix(token, "v") || strings.HasPrefix(token, "V") {
		token = token[1:]
	}
	if !strings.Contains(token, ".") {
		return ""
	}
	if _, ok := CompareVersions(token, token); !ok {
		return ""
	}
	return token
}
