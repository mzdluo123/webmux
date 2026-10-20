//go:build linux || darwin

package session

import "strings"

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

func timerWait(name string) bool {
	name = strings.TrimRight(name, "\x00 ")
	if name == "" || name == "0" {
		return false
	}
	return strings.Contains(name, "nanosleep") || strings.HasPrefix(name, "nanoslee")
}
