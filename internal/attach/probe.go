package attach

import "bytes"

// The session answers the OSC 7501 capability query itself. A native
// terminal that understands the protocol would answer it a second time, so
// the query is withheld from it; status reports still pass through.
var programStatusProbes = [][]byte{[]byte("\x1b]7501;?\a"), []byte("\x1b]7501;?\x1b\\")}

// On a full replay the capability questions describe past output. In
// particular, startup DA1 may already have been answered by the server.
var replayProbes = [][]byte{
	[]byte("\x1b[c"),
	[]byte("\x1b[0c"),
	[]byte("\x1b]11;?\a"),
	[]byte("\x1b]11;?\x1b\\"),
}

// probeFilter removes complete probes from the output stream. A chunk that
// ends in a possible probe prefix holds that prefix back until the next one.
type probeFilter struct {
	held []byte
}

func (f *probeFilter) reset() { f.held = nil }

func (f *probeFilter) filter(p []byte, replay bool) []byte {
	if len(f.held) == 0 && bytes.IndexByte(p, 0x1b) < 0 {
		return p
	}
	in := append(f.held, p...)
	f.held = nil
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); {
		if in[i] != 0x1b {
			next := bytes.IndexByte(in[i:], 0x1b)
			if next < 0 {
				out = append(out, in[i:]...)
				break
			}
			out = append(out, in[i:i+next]...)
			i += next
			continue
		}
		rest := in[i:]
		matched, partial := 0, false
		for _, probe := range programStatusProbes {
			if bytes.HasPrefix(rest, probe) {
				matched = len(probe)
				break
			}
			if len(rest) < len(probe) && bytes.HasPrefix(probe, rest) {
				partial = true
			}
		}
		if replay && matched == 0 {
			for _, probe := range replayProbes {
				if bytes.HasPrefix(rest, probe) {
					matched = len(probe)
					break
				}
			}
			if matched == 0 {
				// Skip payloads such as tmux passthrough, which can contain
				// escaped query bytes that must stay intact.
				n := replayControlStringLen(rest)
				if n != 0 {
					out = append(out, rest[:n]...)
					i += n
					continue
				}
			}
		}
		switch {
		case matched > 0:
			i += matched
		case partial:
			f.held = append([]byte(nil), rest...)
			i = len(in)
		default:
			out = append(out, 0x1b)
			i++
		}
	}
	return out
}

func replayControlStringLen(p []byte) int {
	if len(p) < 2 {
		return 0
	}
	switch p[1] {
	case ']', 'P', 'X', '^', '_':
	default:
		return 0
	}
	for i := 2; i < len(p); i++ {
		switch p[i] {
		case 7:
			if p[1] == ']' {
				return i + 1
			}
		case 27:
			if i+1 < len(p) {
				switch p[i+1] {
				case '\\':
					return i + 2
				case 27:
					i++
				}
			}
		}
	}
	return len(p)
}
