//go:build !linux

package worker

import (
	"log/slog"
	"os/exec"
	"sync"
)

var limitWarningOnce sync.Once

// applySysProcAttr is a no-op on non-Linux platforms.
// exec.CommandContext timeout is still enforced.
func applySysProcAttr(cmd *exec.Cmd, maxMemoryMB int) {
	limitWarningOnce.Do(func() {
		slog.Warn("code execution resource limits are not enforced on this platform — production requires Linux")
	})
	_ = cmd
	_ = maxMemoryMB
}
