package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

// InjectedOBOToken is the corporate on-behalf-of token injected into the sandboxed process.
const InjectedOBOToken = "mock_secret_token_123"

// ParseCommand parses a raw command line string into an executable name and its arguments string.
// It normalizes quoted paths, directory paths, file extensions (e.g., C:\...\pytest.exe -> pytest),
// and recognizes Python module invocations (e.g., python -m pytest -> pytest).
func ParseCommand(cmdStr string) (string, string) {
	trimmed := strings.TrimSpace(cmdStr)
	if trimmed == "" {
		return "", ""
	}

	var rawExec, args string
	if strings.HasPrefix(trimmed, "\"") {
		endIdx := strings.Index(trimmed[1:], "\"")
		if endIdx != -1 {
			rawExec = trimmed[1 : endIdx+1]
			args = strings.TrimSpace(trimmed[endIdx+2:])
		} else {
			rawExec = strings.Trim(trimmed, "\"")
		}
	} else if strings.HasPrefix(trimmed, "'") {
		endIdx := strings.Index(trimmed[1:], "'")
		if endIdx != -1 {
			rawExec = trimmed[1 : endIdx+1]
			args = strings.TrimSpace(trimmed[endIdx+2:])
		} else {
			rawExec = strings.Trim(trimmed, "'")
		}
	} else {
		parts := strings.Fields(trimmed)
		rawExec = parts[0]
		if len(trimmed) > len(rawExec) {
			args = strings.TrimSpace(trimmed[len(rawExec):])
		}
	}

	// Normalize directory separators and extract base executable name
	normalized := strings.ReplaceAll(rawExec, "\\", "/")
	base := path.Base(normalized)

	// Strip .exe extension (case-insensitive)
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, ".exe") {
		base = base[:len(base)-4]
	}

	// Handle "python -m pytest", "python3 -m pytest", or "py -m pytest"
	baseLower := strings.ToLower(base)
	if baseLower == "python" || baseLower == "python3" || baseLower == "py" {
		argParts := strings.Fields(args)
		if len(argParts) >= 2 && argParts[0] == "-m" && argParts[1] == "pytest" {
			return "pytest", args
		}
	}

	return strings.ToLower(base), args
}

// BuildExecCmd creates an *exec.Cmd appropriate for the host OS.
func BuildExecCmd(cmdStr string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
	}
	// Use /bin/sh -c for Linux, macOS, and POSIX
	return exec.Command("/bin/sh", "-c", cmdStr)
}

// CleanOutput normalizes line endings, strips trailing whitespace per line,
// and collapses multiple consecutive blank lines.
func CleanOutput(raw string) string {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	s = strings.Join(lines, "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
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
	return CleanOutput(string(output)), err
}

