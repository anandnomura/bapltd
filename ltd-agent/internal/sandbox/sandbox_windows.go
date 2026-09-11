//go:build windows

package sandbox

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ConfigureSandbox configures sandbox attributes for Windows.
// It isolates process groups, hides console windows, and preserves raw command line formatting.
func ConfigureSandbox(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr.HideWindow = true

	// If cmd is invoking cmd.exe /c, pass the raw command string via CmdLine
	// to prevent Go's makeCmdLine from double-escaping quotes or breaking cmd.exe's parsing.
	if len(cmd.Args) >= 3 && strings.EqualFold(filepath.Base(cmd.Path), "cmd.exe") && cmd.Args[1] == "/c" {
		cmd.SysProcAttr.CmdLine = fmt.Sprintf(`cmd.exe /c %s`, cmd.Args[2])
	}
}


