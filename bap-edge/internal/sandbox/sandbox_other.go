//go:build !linux && !darwin && !windows

package sandbox

import (
	"os/exec"
)

func GetActiveProfile() string {
	return "bap-broker-standard"
}

// ConfigureSandbox fallback for other platforms.
func ConfigureSandbox(cmd *exec.Cmd) func() {
	return func() {}
}

// PostStartProcess fallback for other platforms.
func PostStartProcess(cmd *exec.Cmd) func() {
	return func() {}
}
