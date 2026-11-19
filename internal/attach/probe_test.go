package attach

import (
	"strings"
	"testing"
)

func TestProbeFilter(t *testing.T) {
	for _, row := range []struct{ name, in, want string }{
		{"plain", "hello", "hello"},
		{"bel probe", "a\x1b]7501;?\x07\x1b[cb", "a\x1b[cb"},
		{"st probe", "\x1b]7501;?\x1b\\x", "x"},
		{"report passes", "\x1b]7501;state=working:app=codex\x07", "\x1b]7501;state=working:app=codex\x07"},
		{"reply-shaped passes", "\x1b]7501;?:states=idle\x1b\\", "\x1b]7501;?:states=idle\x1b\\"},
		{"other osc", "\x1b]0;title\x07\x1b[31mred", "\x1b]0;title\x07\x1b[31mred"},
		{"lone escapes", "\x1b\x1b]7501;?\x07\x1b", "\x1b\x1b"},
	} {
		t.Run(row.name, func(t *testing.T) {
			// A trailing possible prefix stays held until more output
			// arrives, so compare what was written plus what is held.
			var whole probeFilter
			if got := string(whole.filter([]byte(row.in), false)) + string(whole.held); got != row.want {
				t.Fatalf("whole: %q, want %q", got, row.want)
			}
			var f probeFilter
			var split strings.Builder
			for i := range row.in {
				split.Write(f.filter([]byte(row.in[i:i+1]), false))
			}
			if got := split.String() + string(f.held); got != row.want {
				t.Fatalf("bytewise: %q, want %q", got, row.want)
			}
		})
	}
}

func TestProbeFilterSuppressesReplayedThemeQueries(t *testing.T) {
	const queries = "\x1b[c\x1b[0c\x1b]11;?\a\x1b]11;?\x1b\\"
	var f probeFilter
	if got := string(f.filter([]byte("before"+queries+"after"), true)); got != "beforeafter" {
		t.Fatalf("replay output = %q", got)
	}
	if got := string(f.filter([]byte(queries), false)); got != queries {
		t.Fatalf("live output = %q", got)
	}
	const passthrough = "\x1bPtmux;\x1b\x1b[c\x1b\\"
	if got := string(f.filter([]byte(passthrough), true)); got != passthrough {
		t.Fatalf("passthrough output = %q", got)
	}
}
