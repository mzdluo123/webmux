package relay

import (
	"os/exec"
	"syscall"
)

func init() {
	configureFRPCParentDeath = func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	}
}
