package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// InjectedOBOToken is the corporate on-behalf-of token injected into the sandboxed process.
const InjectedOBOToken = "mock_secret_token_123"

// ParseCommand parses a raw command line string into an executable name and its arguments string.
func ParseCommand(cmdStr string) (string, string) {
	trimmed := strings.TrimSpace(cmdStr)
	if trimmed == "" {
		return "", ""
	}

	parts := strings.Fields(trimmed)
	executable := parts[0]
	args := ""
	if len(trimmed) > len(executable) {
		args = strings.TrimSpace(trimmed[len(executable):])
	}
	return executable, args
}

// BuildExecCmd creates an *exec.Cmd appropriate for the host OS.
func BuildExecCmd(cmdStr string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		// Use cmd.exe /c for Windows shell commands
		return exec.Command("cmd.exe", "/c", cmdStr)
	}
	// Use /bin/sh -c for Linux, macOS, and POSIX
	return exec.Command("/bin/sh", "-c", cmdStr)
}

// RunSandboxedCommand executes the shell command in an isolated sandbox,
// injecting CORP_OBO_TOKEN and capturing combined stdout and stderr.
func RunSandboxedCommand(cmdStr string) (string, error) {
	cmd := BuildExecCmd(cmdStr)

	// Apply platform-specific sandbox attributes
	ConfigureSandbox(cmd)

	// Inject CORP_OBO_TOKEN environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("CORP_OBO_TOKEN=%s", InjectedOBOToken))

	output, err := cmd.CombinedOutput()
	return string(output), err
}

