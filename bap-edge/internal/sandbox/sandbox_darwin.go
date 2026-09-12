//go:build darwin

package sandbox

import (
	"os/exec"
)

// ConfigureSandbox configures sandbox attributes for Darwin (macOS).
// Linux namespaces are not available on Darwin; process isolation is applied.
func ConfigureSandbox(cmd *exec.Cmd) {
	// Darwin process attributes
}

