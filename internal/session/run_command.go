package session

import "os/exec"

// StartCommand starts cmd under the adopted-process reaper guard and returns a
// wait function that collects its exit status and cleans up any surviving
// processes in the child's session.
func StartCommand(cmd *exec.Cmd) (func() error, error) {
	if err := startChild(cmd); err != nil {
		return nil, err
	}
	pid := cmd.Process.Pid
	return func() error {
		defer releaseLeader(pid)
		err := cmd.Wait()
		_ = CleanupOrphan(pid, "")
		return err
	}, nil
}

// RunCommand protects a service-owned child from the adopted-process reaper
// until exec.Cmd has collected its exit status and finished pipe handling.
func RunCommand(cmd *exec.Cmd) error {
	if err := startChild(cmd); err != nil {
		return err
	}
	defer releaseLeader(cmd.Process.Pid)
	return cmd.Wait()
}
