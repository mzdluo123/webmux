package relaysetting

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/statefile"
)

type machineFile struct {
	V         int                `json:"v"`
	MachineID protocol.MachineID `json:"machineId"`
}

// Machine owns the independent machine identity and its lifetime lock. Opening
// it never creates an identity; ID commits one before allowing enrollment.
// Binding deletion and key rotation must not remove this file.
type Machine struct {
	mu     sync.Mutex
	handle *statefile.Handle[machineFile]
	value  machineFile
	err    error
}

func validateMachine(f machineFile) error {
	if f.V != 1 {
		return errors.New("invalid relay machine identity version")
	}
	return f.MachineID.Validate()
}

// OpenMachine isolates storage failures from the local server. The controller
// can still start, but ID refuses pairing until storage is repaired and reopened.
func OpenMachine(stateDir string) *Machine {
	handle, value, err := statefile.Open(stateDir, statefile.RelayMachine, validateMachine)
	if statefile.IsKind(err, statefile.Missing) {
		err = nil
	}
	return &Machine{handle: handle, value: value, err: err}
}

func (m *Machine) ID() (protocol.MachineID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return "", m.err
	}
	if err := m.handle.Err(); err != nil {
		return "", err
	}
	if m.value.MachineID != "" {
		return m.value.MachineID, nil
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		m.err = errors.New("relay machine identity generation failed")
		return "", m.err
	}
	next := machineFile{V: 1, MachineID: protocol.MachineID(base64.RawURLEncoding.EncodeToString(random[:]))}
	if err := m.handle.Commit(next); err != nil {
		// Never retry an uncertain or failed initialization with another ID.
		m.err = err
		return "", err
	}
	m.value = next
	return next.MachineID, nil
}

func (m *Machine) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handle.Close()
}
