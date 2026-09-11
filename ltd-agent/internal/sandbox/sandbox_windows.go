//go:build windows

package sandbox

import (
	"os/exec"
)

// ConfigureSandbox configures sandbox attributes for Windows.
// Linux namespaces are not available on Windows; standard process isolation is applied.
func ConfigureSandbox(cmd *exec.Cmd) {
	// Windows process attributes
}

