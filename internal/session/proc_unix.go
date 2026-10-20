//go:build linux || darwin

package session

import (
	"path"
	"strings"
)

// state uses ps letters: R running, S sleeping, Z zombie. Other values, including
// '?' for a process whose run state is unreadable, never count as waiting.
// timer is a positive nanosleep or sleep(1) wait, not an unread terminal.
type processState struct {
	ppid  int
	state byte
	sid   int
	start uint64
	timer bool
}
type processTable map[int]processState

func foregroundBlocked(pid int, table processTable) bool {
	root, ok := table[pid]
	if !ok {
		return false
	}
	if root.state == 'R' {
		return false
	}
	children := make(map[int][]int, len(table))
	for child, state := range table {
		children[state.ppid] = append(children[state.ppid], child)
	}
	seen := map[int]bool{pid: true}
	pending := []int{pid}
	leaves := 0
	for len(pending) > 0 {
		member := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		state := table[member]
		if state.state == 'R' {
			return false
		}
		if len(children[member]) == 0 {
			leaves++
			if state.state != 'S' {
				return false
			}
		}
		for _, child := range children[member] {
			// Process snapshots can contain cycles; visit each PID only once.
			if !seen[child] {
				seen[child] = true
				pending = append(pending, child)
			}
		}
	}
	return leaves > 0
}

// awaitingInput is the user-attention half of a quiescent tree. Confirmed
// timer leaves (sleep, nanosleep) are waiting on the clock, not the keyboard.
func awaitingInput(pid int, table processTable) bool {
	if !foregroundBlocked(pid, table) {
		return false
	}
	return !timerLeaves(pid, table)
}

func timerLeaves(pid int, table processTable) bool {
	children := make(map[int][]int, len(table))
	for child, state := range table {
		children[state.ppid] = append(children[state.ppid], child)
	}
	seen := map[int]bool{pid: true}
	pending := []int{pid}
	leaves := 0
	for len(pending) > 0 {
		member := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if len(children[member]) == 0 {
			leaves++
			state := enrichTimer(member, table[member])
			table[member] = state
			if !state.timer {
				return false
			}
		}
		for _, child := range children[member] {
			if !seen[child] {
				seen[child] = true
				pending = append(pending, child)
			}
		}
	}
	return leaves > 0
}

func cString(b []byte) string {
	return cutCString(string(b))
}

func cutCString(s string) string {
	if i := strings.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return strings.TrimRight(s, " ")
}

func sleepCommand(name string) bool {
	return path.Base(cutCString(name)) == "sleep"
}

func timerWait(name string) bool {
	name = cutCString(name)
	if name == "" || name == "0" {
		return false
	}
	// Darwin e_wmesg is 8 bytes with WMESGLEN 7, so nanosleep is stored as "nanosle".
	return strings.Contains(name, "nanosleep") || strings.HasPrefix(name, "nanoslee") || name == "nanosle"
}
