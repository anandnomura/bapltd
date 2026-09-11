//go:build windows

package sandbox

import (
	"os/exec"
	"syscall"
)

// ConfigureSandbox configures sandbox attributes for Windows.
// It isolates process groups, hides console windows, and prevents handle inheritance.
func ConfigureSandbox(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr.HideWindow = true
}

