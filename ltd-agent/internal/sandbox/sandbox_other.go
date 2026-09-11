//go:build !linux && !darwin && !windows

package sandbox

import (
	"os/exec"
)

// ConfigureSandbox fallback for other platforms.
func ConfigureSandbox(cmd *exec.Cmd) {}

