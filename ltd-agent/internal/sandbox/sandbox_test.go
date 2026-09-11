package sandbox

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildExecCmdUsesHostShell(t *testing.T) {
	cmd := BuildExecCmd("echo hello")

	if runtime.GOOS == "windows" {
		if !strings.EqualFold(filepath.Base(cmd.Path), "powershell.exe") {
			t.Fatalf("expected powershell.exe on Windows, got %q", cmd.Path)
		}
		expectedArgs := []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "echo hello"}
		if len(cmd.Args) != len(expectedArgs) {
			t.Fatalf("expected args %v, got %v", expectedArgs, cmd.Args)
		}
		for i := range expectedArgs {
			if cmd.Args[i] != expectedArgs[i] {
				t.Fatalf("expected args %v, got %v", expectedArgs, cmd.Args)
			}
		}
		return
	}

	if cmd.Path != "/bin/sh" {
		t.Fatalf("expected /bin/sh on POSIX, got %q", cmd.Path)
	}
	expectedArgs := []string{"/bin/sh", "-c", "echo hello"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, cmd.Args)
	}
	for i := range expectedArgs {
		if cmd.Args[i] != expectedArgs[i] {
			t.Fatalf("expected args %v, got %v", expectedArgs, cmd.Args)
		}
	}
}

func TestRunSandboxedCommandReturnsExecutionError(t *testing.T) {
	_, err := RunSandboxedCommand("definitely-not-a-real-ltd-agent-command")
	if err == nil {
		t.Fatal("expected command execution error")
	}
}
