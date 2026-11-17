package agentprofile_test

import (
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile"
)

func TestExtractVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "bare", in: "1.5.9\n", want: "1.5.9"},
		{name: "claude", in: "2.1.316 (Claude Code)\n", want: "2.1.316"},
		{name: "codex", in: "codex-cli 0.165.0\n", want: "0.165.0"},
		{name: "omp_slash", in: "omp/18.20.1\n", want: "18.20.1"},
		{name: "v_prefix", in: "opencode v2.1.4\n", want: "2.1.4"},
		{name: "kimi", in: "kimi, version 2.5.2\n", want: "2.5.2"},
		{name: "pep440_post", in: "kimi, version 2.5.2.post1\n", want: "2.5.2.post1"},
		{name: "pep440_rc", in: "3.0.0rc1\n", want: "3.0.0rc1"},
		{name: "pep440_short_rc", in: "v3.0rc2\n", want: "3.0rc2"},
		{name: "devin", in: "Devin CLI 3000.12.57\n", want: "3000.12.57"},
		{name: "key_value", in: "banner line\nversion=1.2.3-beta.1\n", want: "1.2.3-beta.1"},
		{name: "ignore_single_int", in: "Python 3 tool\n", want: ""},
		{name: "ignore_non_version_dots", in: "visit example.com or v1.x or 18.17.x or 1.2.docs or 1.2.c\n", want: ""},
		{name: "empty", in: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := agentprofile.ExtractVersion(tc.in); got != tc.want {
				t.Fatalf("ExtractVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCompareVersionsPEP440AndSemverSuffixes(t *testing.T) {
	orderedPairs := [][2]string{
		{"2.5.2.dev0", "2.5.2a1"},
		{"2.5.2a1", "2.5.2b1"},
		{"2.5.2b1", "2.5.2rc1"},
		{"2.5.2rc1", "2.5.2rc2"},
		{"2.5.2rc2", "2.5.2"},
		{"2.5.2", "2.5.2.post1"},
		{"2.5.2.post1", "2.5.2.post2"},
		{"2.5.2", "3.0.0rc1"},
	}
	for _, pair := range orderedPairs {
		if order, ok := agentprofile.CompareVersions(pair[0], pair[1]); !ok || order >= 0 {
			t.Fatalf("CompareVersions(%q, %q) = (%d, %v), want (<0, true)", pair[0], pair[1], order, ok)
		}
		if order, ok := agentprofile.CompareVersions(pair[1], pair[0]); !ok || order <= 0 {
			t.Fatalf("CompareVersions(%q, %q) = (%d, %v), want (>0, true)", pair[1], pair[0], order, ok)
		}
	}
	if order, ok := agentprofile.CompareVersions("18.13.1-beta.2", "18.13.1"); !ok || order != 0 {
		t.Fatalf("CompareVersions(18.13.1-beta.2, 18.13.1) = (%d, %v), want (0, true)", order, ok)
	}
	for _, invalid := range []string{"", "foo.bar", "1..2", "2.5.2.", "127.0.0.1:8080", "18.17.x", "1.2.docs", "1.2.c", "2.5.2.postx"} {
		if _, ok := agentprofile.CompareVersions(invalid, "1.0.0"); ok {
			t.Fatalf("CompareVersions(%q, 1.0.0) unexpectedly succeeded", invalid)
		}
	}
}
