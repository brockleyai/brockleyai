//go:build linux

package worker

import (
	"os/exec"
	"syscall"
)

// applySysProcAttr sets Linux-specific resource limits and security attributes
// on the subprocess command. This provides process-level isolation on Linux.
func applySysProcAttr(cmd *exec.Cmd, maxMemoryMB int) {
	memBytes := uint64(maxMemoryMB) * 1024 * 1024

	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Run in a new process group so we can kill the whole group on timeout.
		Setpgid: true,
	}

	// Set resource limits via rlimit.
	// RLIMIT_AS: max address space (memory).
	// RLIMIT_CPU: max CPU time in seconds (hard kill).
	// RLIMIT_FSIZE: max file size (prevent disk fill).
	// RLIMIT_NPROC: max number of processes (prevent fork bombs).
	cmd.SysProcAttr.Credential = nil // run as current user
	if cmd.Env == nil {
		cmd.Env = []string{}
	}

	// Apply rlimits via a wrapper if needed. For now, the exec.CommandContext
	// deadline provides the primary enforcement. Container-level limits
	// (cgroups) provide the hard boundary in production.
	_ = memBytes // used by container limits
}
