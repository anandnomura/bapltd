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

	// If invoked via "cmd /c <command>" or "cmd.exe /c <command>", evaluate inner command
	if baseLower == "cmd" {
		argParts := strings.Fields(args)
		if len(argParts) >= 2 && (strings.EqualFold(argParts[0], "/c") || strings.EqualFold(argParts[0], "/k")) {
			innerCmd := strings.TrimSpace(args[len(argParts[0]):])
			return ParseCommand(innerCmd)
		}
	}

	return strings.ToLower(base), args
}

// BuildExecCmd creates an *exec.Cmd appropriate for the host OS.
// On Windows, it uses fast cmd.exe /c (32x faster than powershell) with automatic
// resolution for "ls", shell operators (pipes, chaining, redirection), and dev tools.
func BuildExecCmd(cmdStr string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		trimmed := strings.TrimSpace(cmdStr)
		parts := strings.Fields(trimmed)

		// Direct invocation of powershell or cmd
		if len(parts) > 0 && (parts[0] == "powershell" || parts[0] == "powershell.exe" || parts[0] == "pwsh") {
			return exec.Command(parts[0], parts[1:]...)
		}
		if len(parts) > 0 && (parts[0] == "cmd" || parts[0] == "cmd.exe") {
			return exec.Command("cmd.exe", parts[1:]...)
		}

		// If command uses shell features (pipes, chaining, redirection), execute via cmd.exe /c
		if strings.ContainsAny(trimmed, "|&><;") {
			return exec.Command("cmd.exe", "/c", trimmed)
		}

		// Fast direct resolution for "ls"
		if len(parts) > 0 && parts[0] == "ls" {
			if lsPath, err := exec.LookPath("ls"); err == nil {
				return exec.Command(lsPath, parts[1:]...)
			}
			gitLs := `C:\Program Files\Git\usr\bin\ls.exe`
			if _, err := os.Stat(gitLs); err == nil {
				return exec.Command(gitLs, parts[1:]...)
			}
			if len(parts) == 1 {
				return exec.Command("cmd.exe", "/c", "dir /b")
			}
			return exec.Command("cmd.exe", "/c", "dir /b "+strings.TrimSpace(trimmed[2:]))
		}
		if len(parts) > 0 && parts[0] == "printenv" {
			if pePath, err := exec.LookPath("printenv"); err == nil {
				return exec.Command(pePath, parts[1:]...)
			}
			gitPe := `C:\Program Files\Git\usr\bin\printenv.exe`
			if _, err := os.Stat(gitPe); err == nil {
				return exec.Command(gitPe, parts[1:]...)
			}
		}
		return exec.Command("cmd.exe", "/c", cmdStr)
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

	// Prepare environment
	env := os.Environ()
	if runtime.GOOS == "windows" {
		// Ensure standard utilities from Git (ls, grep, head, tail, etc.) are reachable in cmd.exe
		gitUsrBin := `C:\Program Files\Git\usr\bin`
		if _, err := os.Stat(gitUsrBin); err == nil {
			for i, e := range env {
				if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
					env[i] = e + ";" + gitUsrBin
					break
				}
			}
		}
	}

	// Inject CORP_OBO_TOKEN environment variable
	cmd.Env = append(env, fmt.Sprintf("CORP_OBO_TOKEN=%s", InjectedOBOToken))

	output, err := cmd.CombinedOutput()
	return CleanOutput(string(output)), err
}

