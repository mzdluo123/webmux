package forward

import (
	"slices"

	"github.com/rainchan/webmux/internal/procpeek"
)

// ListeningPortsSupported reports whether the platform supports discovery.
const ListeningPortsSupported = procpeek.ListeningPortsSupported

// ListeningPort is a TCP listener owned by a session process tree.
type ListeningPort = procpeek.ListeningPort

// ListeningPorts returns a caller-owned snapshot of the discovery cache.
func (s *Service) ListeningPorts(id string) []ListeningPort {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.listening[id])
}

// scanPorts runs only while the forward plugin is constructed. Process and
// descriptor I/O never holds the manager, session or forward lock.
func (s *Service) scanPorts(scanner *procpeek.PortScanner) {
	if s.manager == nil {
		return
	}
	infos := s.manager.List()
	roots := make(map[string][]int, len(infos))
	var pids []int
	for _, info := range infos {
		process, err := s.manager.Get(info.ID)
		if err != nil {
			continue
		}
		roots[info.ID] = process.ProcessRoots()
		pids = append(pids, roots[info.ID]...)
	}
	found := scanner.Scan(procpeek.NewSnapshot(procpeek.Processes()), pids)
	next := make(map[string][]ListeningPort, len(infos))
	for _, info := range infos {
		var raw []ListeningPort
		for _, pid := range roots[info.ID] {
			raw = append(raw, found[pid]...)
		}
		next[info.ID] = procpeek.FilterListeningPorts(raw, s.exclude)
	}
	var changed []string
	s.mu.Lock()
	for id, ports := range next {
		if !slices.Equal(s.listening[id], ports) {
			changed = append(changed, id)
		}
	}
	s.listening = next
	s.mu.Unlock()
	// Cache commits precede notifications on the common session event source.
	for _, id := range changed {
		s.manager.Notify(id)
	}
}
